"use client";

import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useState } from "react";

export type AxoMode = "light" | "dark";
export type AxoAppearance = "default" | "warm";
export const APPEARANCE_CHANGED_EVENT = "axo:appearance-changed";
/** Цветовая палитра панели. Coral — из логотипа, mint — историческая зелёная. */
export type AxoPalette = "coral" | "mint";

interface AxoThemeContextValue {
  appearance: AxoAppearance;
  setAppearance: (value: AxoAppearance) => void;
  mode: AxoMode;
  setMode: (m: AxoMode) => void;
  toggleMode: () => void;
  palette: AxoPalette;
  setPalette: (p: AxoPalette) => void;
}

const AxoThemeContext = createContext<AxoThemeContextValue | null>(null);

const STORAGE_KEY = "axo-new-mode";
const APPEARANCE_KEY = "axo-new-appearance";
const PALETTE_KEY = "axo-new-palette";
const DEFAULT_PALETTE: AxoPalette = "coral";

export function AxoThemeProvider({
  children,
  forcedMode,
}: {
  children: React.ReactNode;
  forcedMode?: AxoMode;
}) {
  const [mode, setModeState] = useState<AxoMode>(forcedMode ?? "light");
  const [palette, setPaletteState] = useState<AxoPalette>(DEFAULT_PALETTE);
  const [appearance, setAppearanceState] = useState<AxoAppearance>("default");
  const [ready, setReady] = useState(false);

  useLayoutEffect(() => {
    let next: AxoMode = forcedMode ?? "light";
    try {
      const savedAppearance = localStorage.getItem(APPEARANCE_KEY);
      if (!forcedMode && savedAppearance === "warm") setAppearanceState("warm");
      const saved = localStorage.getItem(STORAGE_KEY) as AxoMode | null;
      const prepaint = document.documentElement.getAttribute("data-axo-mode") as AxoMode | null;
      if (!forcedMode) {
        if (saved === "light" || saved === "dark") next = saved;
        else if (prepaint === "light" || prepaint === "dark") next = prepaint;
        else if (window.matchMedia?.("(prefers-color-scheme: dark)").matches) next = "dark";
      }
    } catch {}
    document.documentElement.setAttribute("data-axo-mode", next);
    document.documentElement.style.backgroundColor = next === "dark" ? "#07090C" : "#F1F6F4";
    document.documentElement.style.colorScheme = next;
    document.body.setAttribute("data-axo-mode", next);
    setModeState(next);
    setReady(true);
  }, [forcedMode]);

  useEffect(() => {
    try {
      const saved = localStorage.getItem(PALETTE_KEY) as AxoPalette | null;
      if (saved === "coral" || saved === "mint") setPaletteState(saved);
    } catch {}
  }, []);

  /* Палитра живёт на <html>: оттуда её видят и порталы (диалоги, тосты,
     выпадающие списки уходят за пределы обёртки приложения), и блокирующий
     скрипт в app/layout.tsx, который ставит атрибут до первой отрисовки.
     Coral — значение по умолчанию в CSS, поэтому атрибут ставится лишь для
     остальных палитр. */
  useEffect(() => {
    if (palette === DEFAULT_PALETTE) {
      document.documentElement.removeAttribute("data-axo-palette");
    } else {
      document.documentElement.setAttribute("data-axo-palette", palette);
    }
    try { localStorage.setItem(PALETTE_KEY, palette); } catch {}
  }, [palette]);

  /* Режим на <html> ставит тот же скрипт до гидратации. Если его не
     обновлять, после переключения темы в интерфейсе html и обёртка начнут
     спорить, и правило от html победит по специфичности. Фон и color-scheme
     пишутся туда же, иначе между гидратацией и первой отрисовкой мелькает
     светлый лист. */
  useEffect(() => {
    if (!ready) return;
    document.documentElement.setAttribute("data-axo-mode", mode);
    document.documentElement.style.backgroundColor = mode === "dark" ? "#07090C" : "#F1F6F4";
    document.documentElement.style.colorScheme = mode;
    document.body.setAttribute("data-axo-mode", mode);
    if (!forcedMode) {
      try { localStorage.setItem(STORAGE_KEY, mode); } catch {}
    }
  }, [forcedMode, mode, ready]);

  useLayoutEffect(() => {
    if (!ready) return;
    document.documentElement.setAttribute("data-axo-appearance", appearance);
    if (!forcedMode) {
      try { localStorage.setItem(APPEARANCE_KEY, appearance); } catch {}
    }
    return () => document.documentElement.removeAttribute("data-axo-appearance");
  }, [appearance, forcedMode, ready]);

  const setAppearance = useCallback((value: AxoAppearance) => {
    if (forcedMode) return;
    setAppearanceState(value);
    // Explicit theme selection takes precedence over a saved preflight experiment.
    window.dispatchEvent(new Event(APPEARANCE_CHANGED_EVENT));
    if (value === "warm") {
      setModeState("dark");
      setPaletteState("coral");
    }
  }, [forcedMode]);

  const setMode = useCallback((m: AxoMode) => {
    if (forcedMode) return;
    setModeState(m);
  }, [forcedMode]);
  const toggleMode = useCallback(() => {
    if (forcedMode) return;
    setModeState((p) => (p === "light" ? "dark" : "light"));
  }, [forcedMode]);
  const setPalette = useCallback((p: AxoPalette) => setPaletteState(p), []);

  return (
    <AxoThemeContext.Provider value={{ appearance, setAppearance, mode, setMode, toggleMode, palette, setPalette }}>
      {children}
    </AxoThemeContext.Provider>
  );
}

export function useAxoTheme() {
  const ctx = useContext(AxoThemeContext);
  if (!ctx) throw new Error("useAxoTheme must be used within AxoThemeProvider");
  return ctx;
}
