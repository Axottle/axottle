"use client";

import { forwardRef, type HTMLAttributes } from "react";
import { motion, type HTMLMotionProps } from "framer-motion";
import { cn } from "./utils";

type Tone = "neutral" | "ok" | "warn" | "crit" | "info" | "accent" | "muted";

const toneClasses: Record<Tone, string> = {
  neutral: "bg-[var(--axo-surface-2)] text-[var(--axo-text)] border-[var(--axo-border)]",
  ok: "bg-[var(--axo-status-ok-soft)] text-[var(--axo-status-ok)] border-transparent",
  warn: "bg-[var(--axo-status-warn-soft)] text-[var(--axo-status-warn)] border-transparent",
  crit: "bg-[var(--axo-status-crit-soft)] text-[var(--axo-status-crit)] border-transparent",
  info: "bg-[var(--axo-status-info-soft)] text-[var(--axo-status-info)] border-transparent",
  accent: "bg-[var(--axo-accent-soft)] text-[var(--axo-accent)] border-transparent",
  muted: "bg-[var(--axo-status-mute-soft)] text-[var(--axo-text-muted)] border-transparent",
};

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: Tone;
  dot?: boolean;
  pulse?: "none" | "ok" | "crit";
  size?: "sm" | "md";
}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(function Badge(
  { className, tone = "neutral", dot, pulse = "none", size = "sm", children, ...rest },
  ref,
) {
  return (
    <span
      ref={ref}
      className={cn(
        "inline-flex items-center gap-1.5 font-medium border whitespace-nowrap",
        size === "sm" ? "h-6 px-2 text-[11px] rounded-[8px]" : "h-7 px-2.5 text-[12px] rounded-[10px]",
        toneClasses[tone],
        className,
      )}
      {...rest}
    >
      {dot && (
        <span
          className={cn(
            "inline-block w-1.5 h-1.5 rounded-full bg-current",
            pulse === "ok" && "axo-pulse-ok",
            pulse === "crit" && "axo-pulse-crit",
          )}
        />
      )}
      {children}
    </span>
  );
});

/* StatusDot — отдельный индикатор состояния для строк/карточек */
export function StatusDot({
  tone = "neutral",
  pulse = "none",
  className,
}: {
  tone?: Tone;
  pulse?: "none" | "ok" | "crit";
  className?: string;
}) {
  const colorMap: Record<Tone, string> = {
    neutral: "bg-[var(--axo-text-faint)]",
    ok: "bg-[var(--axo-status-ok)]",
    warn: "bg-[var(--axo-status-warn)]",
    crit: "bg-[var(--axo-status-crit)]",
    info: "bg-[var(--axo-status-info)]",
    accent: "bg-[var(--axo-accent)]",
    muted: "bg-[var(--axo-status-mute)]",
  };
  return (
    <motion.span
      initial={{ scale: 0.6, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
      className={cn(
        "inline-block w-2 h-2 rounded-full",
        colorMap[tone],
        pulse === "ok" && "axo-pulse-ok",
        pulse === "crit" && "axo-pulse-crit",
        className,
      )}
    />
  );
}
