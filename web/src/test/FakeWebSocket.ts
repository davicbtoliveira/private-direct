export default class FakeWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: FakeWebSocket[] = [];

  readonly url: string;
  readonly requestedProtocols: string[];
  protocol = "";
  readyState = FakeWebSocket.CONNECTING;
  closeCode: number | null = null;
  deferClose = false;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;

  constructor(url: string | URL, protocols: string | string[] = []) {
    this.url = String(url);
    this.requestedProtocols = Array.isArray(protocols) ? protocols : [protocols];
    FakeWebSocket.instances.push(this);
  }

  static reset() {
    FakeWebSocket.instances = [];
  }

  open(protocol = "private-direct") {
    this.protocol = protocol;
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.({ type: "open" } as Event);
  }

  receive(message: unknown) {
    this.onmessage?.({ data: JSON.stringify(message) } as MessageEvent);
  }

  serverClose(code = 1006) {
    this.finishClose(code, false);
  }

  close(code = 1000) {
    this.closeCode = code;
    if (this.deferClose) {
      this.readyState = FakeWebSocket.CLOSING;
      return;
    }
    this.finishClose(code, true);
  }

  completeClose() {
    this.finishClose(this.closeCode ?? 1000, true);
  }

  send() {}

  private finishClose(code: number, wasClean: boolean) {
    if (this.readyState === FakeWebSocket.CLOSED) return;
    this.closeCode = code;
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.({ code, reason: "", type: "close", wasClean } as CloseEvent);
  }
}
