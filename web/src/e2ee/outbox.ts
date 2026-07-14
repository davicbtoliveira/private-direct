import type { EventChain } from "./eventChain";
export type OutboxItem = { id: string; to: number; ciphertext: Record<string, unknown>; chain?:EventChain };
const DB_NAME="private-direct-outbox";const STORE="messages";let memory:OutboxItem[]=[];

function database():Promise<IDBDatabase|null>{
  if (!("indexedDB" in globalThis)) return Promise.resolve(null);
  return new Promise((resolve,reject)=>{const req=indexedDB.open(DB_NAME,1);req.onupgradeneeded=()=>req.result.createObjectStore(STORE,{keyPath:"id"});req.onsuccess=()=>resolve(req.result);req.onerror=()=>reject(req.error)});
}
export async function queueMessage(item:OutboxItem){const db=await database();if(!db){memory=memory.filter(x=>x.id!==item.id);memory.push(item);return}await new Promise<void>((resolve,reject)=>{const tx=db.transaction(STORE,"readwrite");tx.objectStore(STORE).put(item);tx.oncomplete=()=>resolve();tx.onerror=()=>reject(tx.error)});db.close()}
export async function removeQueued(id:string){const db=await database();if(!db){memory=memory.filter(x=>x.id!==id);return}await new Promise<void>((resolve,reject)=>{const tx=db.transaction(STORE,"readwrite");tx.objectStore(STORE).delete(id);tx.oncomplete=()=>resolve();tx.onerror=()=>reject(tx.error)});db.close()}
export async function queuedMessages():Promise<OutboxItem[]>{const db=await database();if(!db)return [...memory];return await new Promise((resolve,reject)=>{const tx=db.transaction(STORE,"readonly");const req=tx.objectStore(STORE).getAll();req.onsuccess=()=>{db.close();resolve(req.result as OutboxItem[])};req.onerror=()=>reject(req.error)})}
export function resetMemoryOutbox(){memory=[]}
