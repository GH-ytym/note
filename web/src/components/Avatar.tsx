import { useState } from "react";

export default function Avatar({ url, name, size = 32 }: { url?: string; name: string; size?: number }) {
  const [failedURL, setFailedURL] = useState("");
  return <span className="user-avatar" style={{ width: size, height: size }} aria-label={`${name}的头像`}>
    {url && url !== failedURL ? <img src={url} alt="" onError={() => setFailedURL(url)} /> : <svg viewBox="0 0 32 32" aria-hidden="true"><circle cx="16" cy="11" r="5" /><path d="M6 28c0-7 4-10 10-10s10 3 10 10" /></svg>}
  </span>;
}
