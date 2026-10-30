"use client";

import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";
import { motion, type HTMLMotionProps } from "framer-motion";
import { cn } from "./utils";

type Variant = "primary" | "secondary" | "ghost" | "outline" | "danger" | "subtle";
type Size = "sm" | "md" | "lg" | "icon";

export interface ButtonProps
  extends Omit<HTMLMotionProps<"button">, "size" | "children"> {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
  leftIcon?: ReactNode;
  rightIcon?: ReactNode;
  children?: ReactNode;
}

const base =
  "axo-focus-ring inline-flex items-center justify-center gap-2 font-medium select-none whitespace-nowrap " +
  "cursor-pointer transition-[background,color,border-color,box-shadow] duration-200 ease-out " +
  "disabled:cursor-not-allowed disabled:opacity-50 disabled:pointer-events-none";

const variants: Record<Variant, string> = {
  primary:
    "bg-[var(--axo-accent)] text-[var(--axo-accent-text)] hover:bg-[var(--axo-accent-hover)] " +
    "shadow-[0_1px_2px_rgba(0,0,0,0.06)] hover:shadow-[var(--axo-shadow-accent-hover)]",
  secondary:
    "bg-[var(--axo-surface-inverse)] text-[var(--axo-text-inverse)] hover:opacity-90 shadow-[0_1px_2px_rgba(0,0,0,0.08)]",
  outline:
    "bg-transparent text-[var(--axo-text)] border border-[var(--axo-border-strong)] hover:bg-[var(--axo-surface-2)]",
  ghost:
    "bg-transparent text-[var(--axo-text)] hover:bg-[var(--axo-surface-2)]",
  subtle:
    "bg-[var(--axo-surface-2)] text-[var(--axo-text)] hover:bg-[var(--axo-surface-3)] border border-[var(--axo-border)]",
  danger:
    "bg-[var(--axo-status-crit)] text-[var(--axo-status-crit-text)] hover:opacity-90 shadow-[0_1px_2px_rgba(0,0,0,0.06)]",
};

// Ниже sm подпись переносится, а высота становится минимальной вместо
// фиксированной. С whitespace-nowrap длинные подписи вроде «Запросить общий
// отчёт Axosun» упирались в край кнопки и обрезались на полуслове. Иконочный
// размер остаётся квадратным — там переносить нечего.
const wrapOnPhone =
  "max-sm:h-auto max-sm:min-h-10 max-sm:py-2 max-sm:whitespace-normal max-sm:text-center max-sm:leading-snug";

const sizes: Record<Size, string> = {
  sm: `h-8 px-3 text-[13px] rounded-[12px] ${wrapOnPhone} max-sm:min-h-9`,
  md: `h-10 px-4 text-sm rounded-[14px] ${wrapOnPhone}`,
  lg: `h-12 px-5 text-[15px] rounded-[16px] ${wrapOnPhone} max-sm:min-h-12`,
  icon: "h-9 w-9 rounded-[12px] shrink-0",
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, variant = "primary", size = "md", loading, leftIcon, rightIcon, children, disabled, ...rest },
  ref,
) {
  return (
    <motion.button
      ref={ref}
      whileTap={{ scale: 0.97 }}
      transition={{ type: "spring", stiffness: 500, damping: 30 }}
      className={cn(base, variants[variant], sizes[size], className)}
      disabled={disabled || loading}
      {...rest}
    >
      {/* Иконки не сжимаются: когда подпись переносится на две строки, флекс
          иначе съедает их ширину и от иконки остаётся полоска. */}
      {loading ? (
        <span className="inline-block w-4 h-4 shrink-0 rounded-full border-2 border-current border-t-transparent animate-spin" />
      ) : (
        leftIcon && <span className="inline-flex shrink-0">{leftIcon}</span>
      )}
      {children}
      {!loading && rightIcon && (
        <span className="inline-flex shrink-0">{rightIcon}</span>
      )}
    </motion.button>
  );
});
