"use client";

import { useId } from "react";
import { motion } from "framer-motion";
import { Check, Minus } from "lucide-react";
import { cn } from "./utils";

/* ────────── Switch ────────── */
export function Switch({
  checked,
  onChange,
  disabled,
  label,
  hint,
  ariaLabel,
  size = "md",
  className,
}: {
  checked: boolean;
  onChange?: (v: boolean) => void;
  disabled?: boolean;
  label?: React.ReactNode;
  hint?: React.ReactNode;
  ariaLabel?: string;
  size?: "sm" | "md";
  className?: string;
}) {
  const w = size === "md" ? 40 : 32;
  const h = size === "md" ? 22 : 18;
  const knob = size === "md" ? 16 : 12;
  const labelID = useId();
  const hintID = useId();
  return (
    <label
      className={cn(
        // The label shares the switch's row; hints must not move its center.
        "inline-grid items-center align-middle cursor-pointer select-none",
        (label || hint) && "grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5",
        disabled && "opacity-50 cursor-not-allowed",
        className,
      )}
    >
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={ariaLabel ?? (typeof label === "string" ? label : undefined)}
        aria-labelledby={!ariaLabel && label && typeof label !== "string" ? labelID : undefined}
        aria-describedby={hint ? hintID : undefined}
        disabled={disabled}
        onClick={() => onChange?.(!checked)}
        className={cn(
          // axo-hit расширяет зону нажатия до 44px на тач-экранах: сам
          // тумблер 40×22 и пальцем по нему промахиваются.
          "axo-hit axo-focus-ring relative shrink-0 rounded-full transition-colors duration-200",
          checked ? "bg-[var(--axo-accent)]" : "bg-[var(--axo-border-strong)]",
        )}
        style={{ width: w, height: h }}
      >
        <motion.span
          layout
          transition={{ type: "spring", stiffness: 700, damping: 35 }}
          className="absolute top-1/2 -translate-y-1/2 rounded-full bg-white shadow-sm"
          style={{
            width: knob,
            height: knob,
            left: checked ? w - knob - 3 : 3,
          }}
        />
      </button>
      {label && <span id={labelID} className="col-start-2 min-w-0 text-[13px] font-medium text-[var(--axo-text)] leading-tight">{label}</span>}
      {hint && <span id={hintID} className="col-start-2 min-w-0 text-[12px] text-[var(--axo-text-muted)] leading-relaxed">{hint}</span>}
    </label>
  );
}

/* ────────── Checkbox ────────── */
export function Checkbox({
  checked,
  indeterminate = false,
  onChange,
  disabled,
  label,
  hint,
  ariaLabel,
  className,
}: {
  checked: boolean;
  indeterminate?: boolean;
  onChange?: (v: boolean) => void;
  disabled?: boolean;
  label?: React.ReactNode;
  hint?: React.ReactNode;
  ariaLabel?: string;
  className?: string;
}) {
  const labelID = useId();
  const hintID = useId();
  return (
    <label
      className={cn(
        "inline-flex items-start gap-2.5 cursor-pointer select-none",
        disabled && "opacity-50 cursor-not-allowed",
        className,
      )}
    >
      <button
        type="button"
        role="checkbox"
        aria-checked={indeterminate ? "mixed" : checked}
        aria-label={ariaLabel ?? (typeof label === "string" ? label : undefined)}
        aria-labelledby={!ariaLabel && label && typeof label !== "string" ? labelID : undefined}
        aria-describedby={hint ? hintID : undefined}
        disabled={disabled}
        onClick={() => onChange?.(indeterminate || !checked)}
        className={cn(
          "axo-hit axo-focus-ring shrink-0 w-5 h-5 rounded-[7px] border transition-colors duration-150 appearance-none shadow-sm",
          "flex items-center justify-center self-start",
          checked || indeterminate
            ? "bg-[var(--axo-accent)] border-[var(--axo-accent)]"
            : "bg-[var(--axo-surface)] border-[var(--axo-border-strong)] hover:border-[var(--axo-accent)] hover:bg-[var(--axo-surface-2)]",
        )}
      >
        {(checked || indeterminate) && (
          <motion.span
            initial={{ scale: 0, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ duration: 0.15 }}
          >
            {indeterminate
              ? <Minus aria-hidden="true" className="w-3 h-3 text-[var(--axo-accent-text)]" strokeWidth={3} />
              : <Check aria-hidden="true" className="w-3 h-3 text-[var(--axo-accent-text)]" strokeWidth={3} />}
          </motion.span>
        )}
      </button>
      {(label || hint) && (
        <span className="flex min-w-0 flex-col gap-0.5">
          {label && <span id={labelID} className="text-[13px] font-medium text-[var(--axo-text)] leading-5">{label}</span>}
          {hint && <span id={hintID} className="text-[12px] text-[var(--axo-text-muted)] leading-relaxed">{hint}</span>}
        </span>
      )}
    </label>
  );
}

/* ────────── Segmented control ────────── */
export function Segmented<T extends string>({
  value,
  onChange,
  options,
  size = "md",
  className,
}: {
  value: T;
  onChange?: (v: T) => void;
  options: { value: T; label: React.ReactNode; icon?: React.ReactNode }[];
  size?: "sm" | "md";
  className?: string;
}) {
  return (
    <div
      className={cn(
        // Набор из четырёх-пяти сегментов в строку на телефоне не влезает —
        // пусть лучше прокручивается, чем растягивает страницу вбок.
        "axo-scroll-x-hidden inline-flex max-w-full items-center gap-1 p-1 rounded-[14px] bg-[var(--axo-surface-2)] border border-[var(--axo-border)]",
        className,
      )}
    >
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange?.(opt.value)}
            className={cn(
              "relative inline-flex shrink-0 items-center gap-1.5 font-medium transition-colors duration-200 rounded-[10px]",
              size === "md"
                ? "h-9 lg:h-8 px-3 text-[13px]"
                : "h-8 lg:h-7 px-2.5 text-[12px]",
              active ? "text-[var(--axo-text)]" : "text-[var(--axo-text-muted)] hover:text-[var(--axo-text)]",
            )}
          >
            {active && (
              <motion.span
                layoutId="axo-segmented-active"
                transition={{ type: "spring", stiffness: 500, damping: 35 }}
                className="absolute inset-0 rounded-[10px] bg-[var(--axo-surface)] shadow-[var(--axo-shadow-sm)] border border-[var(--axo-border)]"
              />
            )}
            <span className="relative inline-flex items-center gap-1.5">
              {opt.icon}
              {opt.label}
            </span>
          </button>
        );
      })}
    </div>
  );
}
