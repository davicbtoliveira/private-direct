const PREFIX = "private-direct-contact-identity:";

function canonical(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value && typeof value === "object") return `{${Object.entries(value as Record<string, unknown>).sort(([a],[b]) => a.localeCompare(b)).map(([key,item]) => `${JSON.stringify(key)}:${canonical(item)}`).join(",")}}`;
  return JSON.stringify(value);
}

export async function identityFingerprint(identity: Record<string, unknown>): Promise<string> {
  const hash = new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(canonical(identity))));
  return Array.from(hash, byte => byte.toString(10).padStart(3, "0")).join("").match(/.{1,5}/g)!.slice(0, 12).join(" ");
}

export function knownIdentity(username: string) { return localStorage.getItem(PREFIX + username); }
export function trustIdentity(username: string, fingerprint: string) { localStorage.setItem(PREFIX + username, fingerprint); }
