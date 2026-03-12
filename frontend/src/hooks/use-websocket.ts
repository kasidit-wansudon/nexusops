"use client";

import { useEffect, useRef, useState, useCallback } from "react";

export interface WebSocketMessage {
  type: string;
  room?: string;
  payload: unknown;
  timestamp?: string;
}

interface UseWebSocketOptions {
  url: string;
  rooms?: string[];
  onMessage?: (message: WebSocketMessage) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
  onError?: (error: Event) => void;
  autoReconnect?: boolean;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  enabled?: boolean;
}

interface UseWebSocketReturn {
  isConnected: boolean;
  lastMessage: WebSocketMessage | null;
  sendMessage: (message: WebSocketMessage) => void;
  subscribe: (room: string) => void;
  unsubscribe: (room: string) => void;
  reconnect: () => void;
  disconnect: () => void;
  reconnectAttempts: number;
}

export function useWebSocket({
  url,
  rooms = [],
  onMessage,
  onConnect,
  onDisconnect,
  onError,
  autoReconnect = true,
  reconnectInterval = 3000,
  maxReconnectAttempts = 10,
  enabled = true,
}: UseWebSocketOptions): UseWebSocketReturn {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const subscribedRoomsRef = useRef<Set<string>>(new Set(rooms));

  const [isConnected, setIsConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  }, []);

  const sendMessage = useCallback((message: WebSocketMessage) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          ...message,
          timestamp: message.timestamp || new Date().toISOString(),
        })
      );
    }
  }, []);

  const subscribe = useCallback(
    (room: string) => {
      subscribedRoomsRef.current.add(room);
      sendMessage({ type: "subscribe", room, payload: {} });
    },
    [sendMessage]
  );

  const unsubscribe = useCallback(
    (room: string) => {
      subscribedRoomsRef.current.delete(room);
      sendMessage({ type: "unsubscribe", room, payload: {} });
    },
    [sendMessage]
  );

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    try {
      const ws = new WebSocket(url);

      ws.onopen = () => {
        setIsConnected(true);
        reconnectAttemptsRef.current = 0;
        setReconnectAttempts(0);
        clearReconnectTimer();

        // Re-subscribe to all rooms
        subscribedRoomsRef.current.forEach((room) => {
          ws.send(
            JSON.stringify({
              type: "subscribe",
              room,
              payload: {},
              timestamp: new Date().toISOString(),
            })
          );
        });

        onConnect?.();
      };

      ws.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          setLastMessage(message);
          onMessage?.(message);
        } catch {
          // Handle non-JSON messages
          const message: WebSocketMessage = {
            type: "raw",
            payload: event.data,
            timestamp: new Date().toISOString(),
          };
          setLastMessage(message);
          onMessage?.(message);
        }
      };

      ws.onerror = (error) => {
        onError?.(error);
      };

      ws.onclose = () => {
        setIsConnected(false);
        wsRef.current = null;
        onDisconnect?.();

        if (
          autoReconnect &&
          reconnectAttemptsRef.current < maxReconnectAttempts
        ) {
          reconnectAttemptsRef.current += 1;
          setReconnectAttempts(reconnectAttemptsRef.current);

          const delay =
            reconnectInterval *
            Math.min(Math.pow(1.5, reconnectAttemptsRef.current), 30);

          reconnectTimerRef.current = setTimeout(() => {
            connect();
          }, delay);
        }
      };

      wsRef.current = ws;
    } catch {
      // Connection failed
      if (
        autoReconnect &&
        reconnectAttemptsRef.current < maxReconnectAttempts
      ) {
        reconnectAttemptsRef.current += 1;
        setReconnectAttempts(reconnectAttemptsRef.current);
        reconnectTimerRef.current = setTimeout(connect, reconnectInterval);
      }
    }
  }, [
    url,
    autoReconnect,
    reconnectInterval,
    maxReconnectAttempts,
    onConnect,
    onDisconnect,
    onMessage,
    onError,
    clearReconnectTimer,
  ]);

  const disconnect = useCallback(() => {
    clearReconnectTimer();
    reconnectAttemptsRef.current = maxReconnectAttempts; // Prevent auto-reconnect
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setIsConnected(false);
  }, [clearReconnectTimer, maxReconnectAttempts]);

  const reconnect = useCallback(() => {
    reconnectAttemptsRef.current = 0;
    setReconnectAttempts(0);
    disconnect();
    // Reset the attempts cap so reconnect works
    reconnectAttemptsRef.current = 0;
    connect();
  }, [connect, disconnect]);

  useEffect(() => {
    if (enabled) {
      connect();
    } else {
      disconnect();
    }

    return () => {
      clearReconnectTimer();
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, url]);

  // Update subscribed rooms when rooms prop changes
  useEffect(() => {
    const currentRooms = subscribedRoomsRef.current;
    const newRooms = new Set(rooms);

    // Subscribe to new rooms
    newRooms.forEach((room) => {
      if (!currentRooms.has(room)) {
        subscribe(room);
      }
    });

    // Unsubscribe from removed rooms
    currentRooms.forEach((room) => {
      if (!newRooms.has(room)) {
        unsubscribe(room);
      }
    });
  }, [rooms, subscribe, unsubscribe]);

  return {
    isConnected,
    lastMessage,
    sendMessage,
    subscribe,
    unsubscribe,
    reconnect,
    disconnect,
    reconnectAttempts,
  };
}

export default useWebSocket;
