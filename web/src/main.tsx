import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import { I18nProvider } from "@/i18n/I18nProvider";
import { SessionProvider } from "@/hooks/useSession";
import "@/styles/theme.css";
import "@/styles/app.css";
import "@/styles/desktop.css";

// Single QueryClient. Retries are minimal because withFallback already handles
// backend-unavailable by serving mocks; we don't want to hammer a down service.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 5_000,
    },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <SessionProvider>
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </SessionProvider>
      </I18nProvider>
    </QueryClientProvider>
  </StrictMode>,
);
