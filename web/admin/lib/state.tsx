"use client";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { useI18n } from "@/lib/i18n";
import {
  APIError,
  request,
  WORKSPACE_KEY,
  type Account,
  type Project,
} from "./api";
export function useText() {
  const { lang } = useI18n();
  return useCallback(
    (ru: string, en: string) => (lang === "ru" ? ru : en),
    [lang],
  );
}
interface Session {
  me: Account | null;
  projects: Project[];
  project: Project | null;
  ready: boolean;
  error: string;
  can: (p: string) => boolean;
  choose: (id: string) => void;
  refresh: () => Promise<void>;
}
const Context = createContext<Session | null>(null);
export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [me, setMe] = useState<Account | null>(null),
    [projects, setProjects] = useState<Project[]>([]),
    [id, setID] = useState(""),
    [ready, setReady] = useState(false),
    [error, setError] = useState("");
  const choose = useCallback((value: string) => {
    setID(value);
    try {
      sessionStorage.setItem(WORKSPACE_KEY, value);
      localStorage.setItem(WORKSPACE_KEY, value);
    } catch {}
  }, []);
  const refresh = useCallback(async () => {
    const data = await request<{ projects: Project[] }>("projects");
    setProjects(data.projects);
    let saved = "";
    try {
      saved =
        sessionStorage.getItem(WORKSPACE_KEY) ||
        localStorage.getItem(WORKSPACE_KEY) ||
        "";
    } catch {}
    choose(
      data.projects.some((p) => p.id === saved)
        ? saved
        : data.projects[0]?.id || "",
    );
  }, [choose]);
  useEffect(() => {
    let current = true;
    const abort = new AbortController();
    (async () => {
      try {
        const account = await request<Account>("auth/me", {
          signal: abort.signal,
        });
        if (!current) return;
        setMe(account);
        await refresh();
      } catch (err) {
        if (!current) return;
        if (err instanceof APIError && err.status === 401)
          window.location.replace("/login/");
        else setError(err instanceof Error ? err.message : String(err));
      } finally {
        if (current) setReady(true);
      }
    })();
    return () => {
      current = false;
      abort.abort();
    };
  }, [refresh]);
  return (
    <Context.Provider
      value={{
        me,
        projects,
        project: projects.find((p) => p.id === id) || null,
        ready,
        error,
        can: (p) => Boolean(me?.permissions.some((v) => v === "*" || v === p)),
        choose,
        refresh,
      }}
    >
      {children}
    </Context.Provider>
  );
}
export function useSession() {
  const v = useContext(Context);
  if (!v) throw new Error("SessionProvider required");
  return v;
}
export function useResource<T>(path: string | null) {
  const { project } = useSession();
  const [data, setData] = useState<T | null>(null),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(Boolean(path)),
    [revision, setRevision] = useState(0);
  useEffect(() => {
    if (!path) {
      setData(null);
      setLoading(false);
      return;
    }
    const abort = new AbortController();
    setLoading(true);
    setError("");
    setData(null);
    request<T>(path, { signal: abort.signal, project: project?.id || "" })
      .then((value) => {
        if (!abort.signal.aborted) setData(value);
      })
      .catch((err) => {
        if (!abort.signal.aborted)
          setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!abort.signal.aborted) setLoading(false);
      });
    return () => abort.abort();
  }, [path, project?.id, revision]);
  return { data, error, loading, refresh: () => setRevision((v) => v + 1) };
}
