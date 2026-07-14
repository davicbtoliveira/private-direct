const DB = "private-direct-secrets";
const STORE = "account-keys";

function database(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB, 1);
    request.onupgradeneeded = () => request.result.createObjectStore(STORE);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

export async function saveMasterKey(username: string, key: CryptoKey): Promise<void> {
  const db = await database();
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite");
    tx.objectStore(STORE).put(key, username);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
  });
  db.close();
}

export async function loadMasterKey(username: string): Promise<CryptoKey | null> {
  const db = await database();
  const value = await new Promise<CryptoKey | undefined>((resolve, reject) => {
    const request = db.transaction(STORE).objectStore(STORE).get(username);
    request.onsuccess = () => resolve(request.result as CryptoKey | undefined);
    request.onerror = () => reject(request.error);
  });
  db.close();
  return value ?? null;
}
