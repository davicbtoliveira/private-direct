import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import FakeWebSocket from "./FakeWebSocket";

globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;

// jsdom does not implement File.prototype.arrayBuffer
if (!File.prototype.arrayBuffer) {
  File.prototype.arrayBuffer = async function () {
    return new Blob([this]).arrayBuffer();
  };
}

// jsdom does not implement Blob.prototype.arrayBuffer either
if (!Blob.prototype.arrayBuffer) {
  Blob.prototype.arrayBuffer = function () {
    return new Promise((resolve) => {
      const reader = new FileReader();
      reader.onloadend = () => resolve(reader.result as ArrayBuffer);
      reader.readAsArrayBuffer(this);
    });
  };
}

afterEach(() => {
  cleanup();
  FakeWebSocket.reset();
});
