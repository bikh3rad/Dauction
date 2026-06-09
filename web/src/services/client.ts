import axios, { AxiosError, type AxiosInstance, type AxiosRequestConfig } from "axios";
import { config } from "./config";
import type { ApiError } from "@/types";

// Centralized axios instance. Auth uses the gateway's dev scheme: the Bearer
// token IS the account UUID (root gateway CLAUDE.md). A real JWT slots in here.
let accountId: string | null = config.devAccountId || null;

export function setAccountId(id: string | null) {
  accountId = id;
}
export function getAccountId(): string | null {
  return accountId;
}

export const http: AxiosInstance = axios.create({
  baseURL: config.apiBase,
  timeout: config.timeoutMs,
  headers: { "Content-Type": "application/json" },
});

http.interceptors.request.use((cfg) => {
  if (accountId) {
    cfg.headers = cfg.headers ?? {};
    cfg.headers.Authorization = `Bearer ${accountId}`;
  }
  return cfg;
});

/** Normalize any axios/network error into our ApiError envelope. */
export function toApiError(err: unknown): ApiError {
  if (axios.isAxiosError(err)) {
    const ax = err as AxiosError<ApiError>;
    if (ax.response?.data && typeof ax.response.data === "object") {
      return {
        message: ax.response.data.message || ax.message,
        details: ax.response.data.details,
        code: ax.response.data.code || String(ax.response.status),
      };
    }
    return { message: ax.message, code: ax.code };
  }
  return { message: err instanceof Error ? err.message : "Unknown error" };
}

/** True when an error means the backend is unreachable (not a 4xx business error). */
export function isUnavailable(err: unknown): boolean {
  if (axios.isAxiosError(err)) {
    if (!err.response) return true; // network / timeout / DNS / refused
    // 5xx and 502/503/504 gateway-upstream errors → backend effectively down
    if (err.response.status >= 500) return true;
  }
  return false;
}

// thin verb helpers returning parsed JSON bodies
export async function get<T>(url: string, cfg?: AxiosRequestConfig): Promise<T> {
  return (await http.get<T>(url, cfg)).data;
}
export async function post<T>(url: string, body?: unknown, cfg?: AxiosRequestConfig): Promise<T> {
  return (await http.post<T>(url, body, cfg)).data;
}
