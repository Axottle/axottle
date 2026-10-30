"use client";
import { AxoThemeProvider } from "@/app/new/theme";
import "@/app/new/theme.css";
export default function SetupLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AxoThemeProvider forcedMode="dark">
      <div data-axo-new className="min-h-dvh">
        {children}
      </div>
    </AxoThemeProvider>
  );
}
