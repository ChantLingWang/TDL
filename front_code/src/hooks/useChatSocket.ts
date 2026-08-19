// ============================================================================
// WebSocket 生命周期管理（连接、断线重连、统一消息出口）
// ChatPage 只消费「消息回调」，不再关心连接细节。
// ============================================================================
import { useCallback, useEffect, useRef, useState } from 'react';
import type { WsMessage } from '../types/chat';

const WS_BASE = import.meta.env.VITE_WS_URL
  || (window.location.protocol === 'https:' ? 'wss' : 'ws') + '://' + window.location.host + '/api/v1/ws';

interface UseChatSocketOptions {
  token: string;
  /** 每收到一条服务端消息（含阶段三的 delta 分块）统一走这里 */
  onMessage: (msg: WsMessage) => void;
}

interface UseChatSocketResult {
  connected: boolean;
  statusMsg: string;
  /** 发送聊天消息；连接未就绪时返回 false（由调用方提示） */
  send: (payload: object) => boolean;
}

export function useChatSocket({ token, onMessage }: UseChatSocketOptions): UseChatSocketResult {
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);
  const [statusMsg, setStatusMsg] = useState('connecting...');
  // 最新回调始终可见，避免闭包过期
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  const connectWS = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN ||
        wsRef.current?.readyState === WebSocket.CONNECTING) return;
    if (wsRef.current) wsRef.current.onclose = null;

    const ws = new WebSocket(WS_BASE + '?token=' + encodeURIComponent(token));
    wsRef.current = ws;

    ws.onopen = () => { setConnected(true); setStatusMsg('online'); };
    ws.onmessage = (ev) => {
      try {
        const msg: WsMessage = JSON.parse(ev.data);
        onMessageRef.current(msg);
      } catch { /* 忽略坏包 */ }
    };
    ws.onclose = () => {
      setConnected(false);
      setStatusMsg('已断开，2 秒后重连...');
      setTimeout(connectWS, 2000);
    };
    ws.onerror = () => {};
  }, [token]);

  useEffect(() => {
    connectWS();
    return () => {
      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.close();
      }
    };
  }, [connectWS]);

  const send = useCallback((payload: object): boolean => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return false;
    ws.send(JSON.stringify(payload));
    return true;
  }, []);

  return { connected, statusMsg, send };
}
