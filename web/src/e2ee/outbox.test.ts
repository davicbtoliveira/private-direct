import { beforeEach, expect, it } from "vitest";
import { queueMessage, queuedMessages, removeQueued, resetMemoryOutbox } from "./outbox";

beforeEach(resetMemoryOutbox);

it("queues only ciphertext and removes committed messages", async()=>{
  await queueMessage({id:"message-one",to:2,ciphertext:{algorithm:"m.megolm.v1.aes-sha2",ciphertext:"opaque"}});
  const queued=await queuedMessages();
  expect(queued).toHaveLength(1);
  expect(JSON.stringify(queued)).not.toContain("plaintext secret");
  await removeQueued("message-one");
  expect(await queuedMessages()).toEqual([]);
});
