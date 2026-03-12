"use client";

import { useState, useCallback, useRef, useEffect } from "react";

interface ApiState<T> {
  data: T | null;
  error: string | null;
  loading: boolean;
}

interface UseApiOptions {
  baseUrl?: string;
  headers?: Record<string, string>;
  onError?: (error: string) => void;
  onUnauthorized?: () => void;
}

interface RequestOptions {
  headers?: Record<string, string>;
  signal?: AbortSignal;
  params?: Record<string, string | number | boolean | undefined>;
}

function buildUrl(
  base: string,
  path: string,
  params?: Record<string, string | number | boolean | undefined>
): string {
  const url = new URL(path, base);
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined) {
        url.searchParams.set(key, String(value));
      }
    });
  }
  return url.toString();
}

export function useApi<T = unknown>(options: UseApiOptions = {}) {
  const {
    baseUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api",
    headers: defaultHeaders = {},
    onError,
    onUnauthorized,
  } = options;

  const [state, setState] = useState<ApiState<T>>({
    data: null,
    error: null,
    loading: false,
  });

  const abortControllerRef = useRef<AbortController | null>(null);

  const getAuthHeaders = useCallback((): Record<string, string> => {
    const token =
      typeof window !== "undefined"
        ? localStorage.getItem("nexusops_token")
        : null;
    return token ? { Authorization: `Bearer ${token}` } : {};
  }, []);

  const request = useCallback(
    async (
      method: string,
      path: string,
      body?: unknown,
      requestOptions?: RequestOptions
    ): Promise<T> => {
      // Cancel previous request if still in flight
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }

      const controller = new AbortController();
      abortControllerRef.current = controller;

      setState((prev) => ({ ...prev, loading: true, error: null }));

      try {
        const url = buildUrl(baseUrl, path, requestOptions?.params);
        const response = await fetch(url, {
          method,
          headers: {
            "Content-Type": "application/json",
            ...defaultHeaders,
            ...getAuthHeaders(),
            ...requestOptions?.headers,
          },
          body: body ? JSON.stringify(body) : undefined,
          signal: requestOptions?.signal || controller.signal,
        });

        if (response.status === 401) {
          onUnauthorized?.();
          throw new Error("Unauthorized");
        }

        if (!response.ok) {
          const errorBody = await response.json().catch(() => ({}));
          const errorMessage =
            (errorBody as { message?: string }).message ||
            `Request failed with status ${response.status}`;
          throw new Error(errorMessage);
        }

        // Handle 204 No Content
        if (response.status === 204) {
          const data = null as unknown as T;
          setState({ data, error: null, loading: false });
          return data;
        }

        const data = (await response.json()) as T;
        setState({ data, error: null, loading: false });
        return data;
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") {
          // Request was cancelled, don't update state
          throw err;
        }

        const errorMessage =
          err instanceof Error ? err.message : "An unexpected error occurred";
        setState({ data: null, error: errorMessage, loading: false });
        onError?.(errorMessage);
        throw err;
      }
    },
    [baseUrl, defaultHeaders, getAuthHeaders, onError, onUnauthorized]
  );

  const get = useCallback(
    (path: string, options?: RequestOptions) =>
      request("GET", path, undefined, options),
    [request]
  );

  const post = useCallback(
    (path: string, body?: unknown, options?: RequestOptions) =>
      request("POST", path, body, options),
    [request]
  );

  const put = useCallback(
    (path: string, body?: unknown, options?: RequestOptions) =>
      request("PUT", path, body, options),
    [request]
  );

  const patch = useCallback(
    (path: string, body?: unknown, options?: RequestOptions) =>
      request("PATCH", path, body, options),
    [request]
  );

  const del = useCallback(
    (path: string, options?: RequestOptions) =>
      request("DELETE", path, undefined, options),
    [request]
  );

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, []);

  return {
    ...state,
    get,
    post,
    put,
    patch,
    del,
    request,
    reset: () => setState({ data: null, error: null, loading: false }),
  };
}

export default useApi;
