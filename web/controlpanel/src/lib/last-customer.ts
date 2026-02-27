const KEY = "lastCustomerId";

export function getLastCustomerId(): string | null {
  return localStorage.getItem(KEY);
}

export function setLastCustomerId(id: string): void {
  localStorage.setItem(KEY, id);
}

export function clearLastCustomerId(): void {
  localStorage.removeItem(KEY);
}
