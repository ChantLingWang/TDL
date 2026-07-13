/** 解码 JWT payload，不验证签名 */
export function parseJwt(token: string): Record<string, any> | null {
  try {
    const base64 = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/');
    return JSON.parse(atob(base64));
  } catch { return null; }
}

/** 检查 token 是否在 minutes 分钟内过期 */
export function expiresWithin(token: string, minutes = 5): boolean {
  const payload = parseJwt(token);
  if (!payload?.exp) return true;
  return (payload.exp * 1000) - Date.now() < minutes * 60_000;
}
