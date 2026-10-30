"use client";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { I18nProvider, useI18n } from "@/lib/i18n";
import { request } from "./api";
interface Presentation {
  name: string;
  language: string;
  timezone: string;
  refresh: () => Promise<void>;
}
const Context = createContext<Presentation>({
  name: "Axottle",
  language: "ru",
  timezone: "UTC",
  refresh: async () => {},
});
function PresentationProvider({ children }: { children: React.ReactNode }) {
  const [value, setValue] = useState({
      name: "Axottle",
      language: "ru",
      timezone: "UTC",
    }),
    { setLang } = useI18n(),
    applied = useRef(false);
  const [saved] = useState(() => {
    try {
      return (
        typeof window !== "undefined" &&
        Boolean(localStorage.getItem("axottle-lang"))
      );
    } catch {
      return false;
    }
  });
  const refresh = useCallback(async () => {
    const next = await request<{
      name: string;
      language: string;
      timezone: string;
    }>("core/presentation", { project: "" });
    setValue(next);
    document.title =
      next.name === "Axottle" ? "Axottle" : `${next.name} · Axottle`;
    if (!applied.current) {
      applied.current = true;
      if (!saved && (next.language === "ru" || next.language === "en"))
        setLang(next.language);
    }
  }, [saved, setLang]);
  useEffect(() => {
    void refresh().catch(() => {});
  }, [refresh]);
  return (
    <Context.Provider value={{ ...value, refresh }}>
      {children}
    </Context.Provider>
  );
}
export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <I18nProvider>
      <PresentationProvider>{children}</PresentationProvider>
    </I18nProvider>
  );
}
export function usePresentation() {
  return useContext(Context);
}
