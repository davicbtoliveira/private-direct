export type PeerChannelState = "idle" | "negotiating" | "directly-connected" | "offline" | "failed";

type IceServerConfig = {
  urls: string[];
  username?: string;
  credential?: string;
};

interface PeerState {
  connection: RTCPeerConnection | null;
  channel: RTCDataChannel | null;
  state: PeerChannelState;
  connectionId: string | null;
  polite: boolean;
  makingOffer: boolean;
  ignoreOffer: boolean;
  pendingIceCandidates: RTCIceCandidateInit[];
  negotiationTimer: ReturnType<typeof setTimeout> | null;
  autoRetryUsed: boolean;
  pendingMessages: string[];
}

export type SignalSender = (toUserId: number, signalType: string, payload: Record<string, unknown>) => void;
type StateCallback = (userId: number, state: PeerChannelState) => void;
type MessageCallback = (fromUserId: number, data: string) => void;
type IsOnlineFn = (userId: number) => boolean;

const NEGOTIATION_TIMEOUT_MS = 15_000;

export class PeerManager {
  private peers = new Map<number, PeerState>();
  private myUserId = 0;
  private iceServers: RTCIceServer[] = [];
  private sendSignal: SignalSender | null = null;
  private onStateChange: StateCallback | null = null;
  private onMessage: MessageCallback | null = null;
  private isOnline: IsOnlineFn | null = null;

  init(
    myUserId: number,
    iceServers: IceServerConfig[],
    sendSignal: SignalSender,
    onStateChange: StateCallback,
    onMessage: MessageCallback,
    isOnline: IsOnlineFn,
  ) {
    this.myUserId = myUserId;
    this.iceServers = iceServers.map((s) => ({
      urls: Array.isArray(s.urls) ? s.urls : [s.urls as unknown as string],
      ...(s.username ? { username: s.username, credential: s.credential ?? "" } : {}),
    }));
    this.sendSignal = sendSignal;
    this.onStateChange = onStateChange;
    this.onMessage = onMessage;
    this.isOnline = isOnline;
  }

  connect(userId: number) {
    if (!this.isOnline || !this.isOnline(userId)) {
      this.setState(userId, "offline");
      return;
    }
    const existing = this.peers.get(userId);
    if (existing?.state === "directly-connected") return;
    if (existing?.state === "negotiating") return;
    this.createPeerConnection(userId);
  }

  disconnect(userId: number) {
    this.closePeer(userId);
  }

  disconnectAll() {
    for (const userId of this.peers.keys()) {
      this.closePeer(userId);
    }
    this.peers.clear();
  }

  handleSignal(fromUserId: number, signalType: string, payload: Record<string, unknown>) {
    const state = this.peers.get(fromUserId);
    if (!state) {
      const newState = this.createPeerConnection(fromUserId);
      if (!newState) return;
      this.applySignal(newState, signalType, payload);
      return;
    }
    this.applySignal(state, signalType, payload);
  }

  handleSignalError(_error: string) {
  }

  private applySignal(state: PeerState, signalType: string, payload: Record<string, unknown>) {
    const connectionId = payload.connection_id as string | undefined;
    if (connectionId && state.connectionId && state.connectionId !== connectionId) {
      return;
    }
    if (connectionId) {
      state.connectionId = connectionId;
    }

    if (signalType === "offer") {
      this.handleOffer(state, payload);
    } else if (signalType === "answer") {
      this.handleAnswer(state, payload);
    } else if (signalType === "ice") {
      this.handleIce(state, payload);
    }
  }

  sendMessage(userId: number, data: string) {
    const state = this.peers.get(userId);
    if (!state) return false;
    if (state.channel && state.channel.readyState === "open") {
      state.channel.send(data);
      return true;
    }
    if (state.pendingMessages.length < 20) {
      state.pendingMessages.push(data);
      return true;
    }
    return false;
  }

  getState(userId: number): PeerChannelState {
    return this.peers.get(userId)?.state ?? "idle";
  }

  getChannel(userId: number): RTCDataChannel | null {
    return this.peers.get(userId)?.channel ?? null;
  }

  retryConnection(userId: number) {
    this.closePeer(userId);
    const state = this.peers.get(userId);
    if (state) {
      state.autoRetryUsed = false;
    }
    if (this.isOnline?.(userId)) {
      this.createPeerConnection(userId);
    }
  }

  private setState(userId: number, nextState: PeerChannelState) {
    let state = this.peers.get(userId);
    if (!state) return;
    state.state = nextState;
    this.onStateChange?.(userId, nextState);
  }

  private handleOffer(state: PeerState, payload: Record<string, unknown>) {
    if (!state.connection) return;
    const userId = this.getUserIdFromState(state);
    const description = { type: "offer" as const, sdp: payload.sdp as string };
    const isStable =
      state.connection.signalingState === "stable" ||
      (state.connection.signalingState === "have-local-offer" && state.makingOffer);
    state.ignoreOffer = !state.polite && !isStable;
    if (state.ignoreOffer) return;

    clearNegotiationTimer(state);
    startNegotiationTimer(this, state);
    state.connection
      .setRemoteDescription(new RTCSessionDescription(description))
      .then(() => this.applyQueuedIce(state))
      .then(() => {
        if (state.connection?.signalingState !== "have-remote-offer") return;
        return state.connection.createAnswer();
      })
      .then((answer) => {
        if (!answer) return;
        return state.connection!.setLocalDescription(answer);
      })
      .then(() => {
        if (!state.connection?.localDescription) return;
        this.sendSignal?.(userId, "answer", {
          sdp: state.connection.localDescription.sdp,
          connection_id: state.connectionId,
        });
      })
      .catch(() => this.handleNegotiationFailure(state));
  }

  private handleAnswer(state: PeerState, payload: Record<string, unknown>) {
    if (!state.connection) return;
    if (state.connection.signalingState !== "have-local-offer") return;
    const description = { type: "answer" as const, sdp: payload.sdp as string };
    state.connection
      .setRemoteDescription(new RTCSessionDescription(description))
      .then(() => this.applyQueuedIce(state))
      .catch(() => this.handleNegotiationFailure(state));
  }

  private handleIce(state: PeerState, payload: Record<string, unknown>) {
    if (!state.connection) return;
    const candidate: RTCIceCandidateInit = {
      candidate: payload.candidate as string,
      sdpMid: (payload.sdp_mid ?? null) as string | null,
      sdpMLineIndex: (payload.sdp_m_line_index ?? null) as number | null,
    };
    if (!state.connection.remoteDescription) {
      state.pendingIceCandidates.push(candidate);
      return;
    }
    state.connection.addIceCandidate(new RTCIceCandidate(candidate)).catch(() => {});
  }

  private applyQueuedIce(state: PeerState): Promise<void> {
    if (state.pendingIceCandidates.length === 0) return Promise.resolve();
    const pending = state.pendingIceCandidates.splice(0);
    return Promise.all(
      pending.map((c) =>
        state.connection!.addIceCandidate(new RTCIceCandidate(c)).catch(() => {}),
      ),
    ).then(() => undefined);
  }

  private createPeerConnection(userId: number): PeerState | null {
    if (!this.iceServers.length) return null;
    const myId = this.myUserId;
    const polite = myId > userId;
    const connectionId = crypto.randomUUID();

    const pc = new RTCPeerConnection({ iceServers: this.iceServers });
    const state: PeerState = {
      connection: pc,
      channel: null,
      state: "negotiating",
      connectionId,
      polite,
      makingOffer: false,
      ignoreOffer: false,
      pendingIceCandidates: [],
      negotiationTimer: null,
      autoRetryUsed: false,
      pendingMessages: [],
    };

    this.peers.set(userId, state);
    this.setState(userId, "negotiating");

    pc.onicecandidate = (event) => {
      if (!event.candidate) return;
      const payload: Record<string, unknown> = {
        candidate: event.candidate.candidate,
        sdp_mid: event.candidate.sdpMid,
        sdp_m_line_index: event.candidate.sdpMLineIndex,
        connection_id: connectionId,
      };
      this.sendSignal?.(userId, "ice", payload);
    };

    pc.oniceconnectionstatechange = () => {
      if (pc.iceConnectionState === "connected" || pc.iceConnectionState === "completed") {
        clearNegotiationTimer(state);
        this.setState(userId, "directly-connected");
      } else if (
        pc.iceConnectionState === "failed" ||
        pc.iceConnectionState === "disconnected"
      ) {
        this.handleConnectionLoss(userId, state);
      }
    };

    pc.ondatachannel = (event) => {
      const dataChannel = event.channel;
      if (dataChannel.label === "chat") {
        state.channel = dataChannel;
        this.setupDataChannel(userId, dataChannel);
      }
    };

    pc.onnegotiationneeded = async () => {
      state.makingOffer = true;
      try {
        await pc.setLocalDescription();
        this.sendSignal?.(userId, "offer", {
          sdp: pc.localDescription!.sdp,
          connection_id: connectionId,
        });
      } catch {
        this.handleNegotiationFailure(state);
      } finally {
        state.makingOffer = false;
      }
    };

    if (polite) {
      const channel = pc.createDataChannel("chat", {
        ordered: true,
        id: 0,
        negotiated: false,
      });
      state.channel = channel;
      this.setupDataChannel(userId, channel);
    }

    if (!state.polite) {
      startNegotiationTimer(this, state);
    }

    return state;
  }

  private setupDataChannel(userId: number, channel: RTCDataChannel) {
    channel.onopen = () => {
      clearNegotiationTimer(this.peers.get(userId));
      this.setState(userId, "directly-connected");
      const state = this.peers.get(userId);
      if (!state) return;
      const pending = state.pendingMessages.splice(0);
      for (const msg of pending) {
        if (channel.readyState === "open") {
          channel.send(msg);
        }
      }
    };
    channel.onclose = () => {
      const state = this.peers.get(userId);
      if (state?.state === "directly-connected") {
        this.handleConnectionLoss(userId, state);
      }
    };
    channel.onmessage = (event) => {
      this.onMessage?.(userId, String(event.data));
    };
  }

  private handleConnectionLoss(userId: number, state: PeerState) {
    if (!this.isOnline?.(userId)) {
      this.setState(userId, "offline");
      return;
    }
    if (!state.autoRetryUsed) {
      state.autoRetryUsed = true;
      this.closePeer(userId);
      this.createPeerConnection(userId);
    } else {
      this.setState(userId, "failed");
    }
  }

  private handleNegotiationFailure(state: PeerState) {
    clearNegotiationTimer(state);
    state.pendingMessages = [];
    this.setState(this.getUserIdFromState(state), "failed");
    if (state.connection) {
      state.connection.close();
    }
  }

  private closePeer(userId: number) {
    const state = this.peers.get(userId);
    if (!state) return;
    clearNegotiationTimer(state);
    if (state.channel) {
      state.channel.onopen = null;
      state.channel.onclose = null;
      state.channel.onmessage = null;
      state.channel.close();
    }
    if (state.connection) {
      state.connection.onicecandidate = null;
      state.connection.oniceconnectionstatechange = null;
      state.connection.ondatachannel = null;
      state.connection.onnegotiationneeded = null;
      state.connection.close();
    }
  }

  private getUserIdFromState(state: PeerState): number {
    for (const [id, s] of this.peers) {
      if (s === state) return id;
    }
    return 0;
  }
}

function startNegotiationTimer(mgr: PeerManager, state: PeerState) {
  clearNegotiationTimer(state);
  state.negotiationTimer = setTimeout(() => {
    mgr["handleNegotiationFailure"](state);
  }, NEGOTIATION_TIMEOUT_MS);
}

function clearNegotiationTimer(state?: PeerState | null) {
  if (state?.negotiationTimer) {
    clearTimeout(state.negotiationTimer);
    state.negotiationTimer = null;
  }
}
