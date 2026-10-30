"use client";

import { useEffect } from "react";
import { AxoThemeProvider } from "@/app/new/theme";
import "@/app/new/theme.css";

function LoginShell({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    document.body.setAttribute("data-axo-mode", "dark");
    return () => {
      document.body.removeAttribute("data-axo-mode");
    };
  }, []);
  return (
    <div data-axo-new data-axo-mode="dark" className="min-h-dvh w-full bg-[#190d15]">
      {children}
    </div>
  );
}

export default function LoginLayout({ children }: { children: React.ReactNode }) {
  return (
    <AxoThemeProvider forcedMode="dark">
      <LoginShell>{children}</LoginShell>
    </AxoThemeProvider>
  );
}
