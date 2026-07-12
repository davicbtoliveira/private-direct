import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import FakeWebSocket from "./FakeWebSocket";

globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;

afterEach(() => {
  cleanup();
  FakeWebSocket.reset();
});
