"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import {
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Info,
  X,
} from "lucide-react";
import { useAxoTheme } from "@/app/new/theme";
import { useI18n } from "@/lib/i18n";

export type ToastTone = "ok" | "warn" | "crit" | "info";

export interface ToastItem {
  id: string;
  message: React.ReactNode;
  tone: ToastTone;
  /** Auto-dismiss timeout in ms. Set to 0 to disable auto-dismiss. */
  duration?: number;
}

interface ShowOptions {
  tone?: ToastTone;
  duration?: number;
}

interface ToastContextValue {
  show: (message: React.ReactNode, opts?: ShowOptions) => string;
  success: (message: React.ReactNode, opts?: Omit<ShowOptions, "tone">) => string;
  error: (message: React.ReactNode, opts?: Omit<ShowOptions, "tone">) => string;
  warn: (message: React.ReactNode, opts?: Omit<ShowOptions, "tone">) => string;
  info: (message: React.ReactNode, opts?: Omit<ShowOptions, "tone">) => string;
  dismiss: (id: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

const DEFAULT_DURATION = 4000;

const toneStyles: Record<ToastTone, { accent: string; soft: string; Icon: typeof CheckCircle2 }> = {
  ok:   { accent: "var(--axo-status-ok)",   soft: "var(--axo-status-ok-soft)",   Icon: CheckCircle2 },
  warn: { accent: "var(--axo-status-warn)", soft: "var(--axo-status-warn-soft)", Icon: AlertTriangle },
  crit: { accent: "var(--axo-status-crit)", soft: "var(--axo-status-crit-soft)", Icon: XCircle },
  info: { accent: "var(--axo-status-info)", soft: "var(--axo-status-info-soft)", Icon: Info },
};

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const timers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const counter = useRef(0);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    return () => {
      // Clear pending timers on unmount.
      timers.current.forEach((t) => clearTimeout(t));
      timers.current.clear();
    };
  }, []);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
    const timer = timers.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.current.delete(id);
    }
  }, []);

  const show = useCallback(
    (message: React.ReactNode, opts?: ShowOptions): string => {
      const id = `toast-${++counter.current}-${Date.now()}`;
      const item: ToastItem = {
        id,
        message,
        tone: opts?.tone ?? "info",
        duration: opts?.duration ?? DEFAULT_DURATION,
      };
      setToasts((prev) => [...prev, item]);
      if (item.duration && item.duration > 0 && typeof window !== "undefined") {
        const handle = setTimeout(() => dismiss(id), item.duration);
        timers.current.set(id, handle);
      }
      return id;
    },
    [dismiss],
  );

  const value = useMemo<ToastContextValue>(
    () => ({
      show,
      success: (m, o) => show(m, { ...o, tone: "ok" }),
      error: (m, o) => show(m, { ...o, tone: "crit" }),
      warn: (m, o) => show(m, { ...o, tone: "warn" }),
      info: (m, o) => show(m, { ...o, tone: "info" }),
      dismiss,
    }),
    [show, dismiss],
  );

  return (
    <ToastContext.Provider value={value}>
      {children}
      {mounted &&
        typeof document !== "undefined" &&
        createPortal(<ToastViewport toasts={toasts} onDismiss={dismiss} />, document.body)}
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast must be used within a ToastProvider");
  }
  return ctx;
}

function ToastViewport({
  toasts,
  onDismiss,
}: {
  toasts: ToastItem[];
  onDismiss: (id: string) => void;
}) {
  const { t } = useI18n();
  const { mode } = useAxoTheme();
  return (
    <div
      data-axo-new
      data-axo-mode={mode}
      data-axo-toast-viewport
      className="fixed inset-x-3 top-3 z-[110] flex flex-col gap-2 pointer-events-none sm:inset-x-auto sm:right-4 sm:top-4 sm:w-[420px]"
      aria-live="polite"
      aria-atomic="false"
    >
      <AnimatePresence initial={false}>
        {toasts.map((item) => {
          const style = toneStyles[item.tone];
          const Icon = style.Icon;
          return (
            <motion.div
              key={item.id}
              initial={{ opacity: 0, y: -8, scale: 0.98 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -8, scale: 0.98 }}
              transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
              className="pointer-events-auto relative overflow-hidden rounded-[14px] border border-[var(--axo-border)] bg-[var(--axo-surface)] shadow-[var(--axo-shadow-md)]"
              role="status"
            >
              <div className="flex items-start gap-3 px-3.5 py-3">
                <span
                  className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-[10px]"
                  style={{ backgroundColor: style.soft, color: style.accent }}
                  aria-hidden="true"
                >
                  <Icon className="w-4 h-4" />
                </span>
                <div className="flex-1 min-w-0 text-[13px] leading-relaxed text-[var(--axo-text)] break-words">
                  {item.message}
                </div>
                <button
                  type="button"
                  onClick={() => onDismiss(item.id)}
                  className="axo-focus-ring -mr-1 -mt-1 flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center rounded-[10px] text-[var(--axo-text-muted)] hover:bg-[var(--axo-surface-2)] hover:text-[var(--axo-text)] transition-colors duration-200 ease-out"
                  aria-label={t("common.close")}
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>
            </motion.div>
          );
        })}
      </AnimatePresence>
    </div>
  );
}
