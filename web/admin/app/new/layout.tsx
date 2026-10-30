"use client";
import { AxoThemeProvider } from "./theme";
import { ToastProvider } from "@/components/new/toast";
import { SessionProvider } from "@/lib/state";
import { Shell } from "@/components/shell";
import "./theme.css";
import "@/components/core.css";
export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <AxoThemeProvider>
      <ToastProvider>
        <SessionProvider>
          <Shell>{children}</Shell>
        </SessionProvider>
      </ToastProvider>
    </AxoThemeProvider>
  );
}
