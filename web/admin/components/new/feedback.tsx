"use client";

import { LayoutGroup, motion, useReducedMotion } from "framer-motion";
import Link from "next/link";
import { useEffect, useId, useRef } from "react";
import { cn } from "./utils";

/* ────────── Tabs ────────── */
export function Tabs<T extends string>({
  value,
  onChange,
  items,
  className,
}: {
  value: T;
  onChange?: (v: T) => void;
  items: {
    value: T;
    label: React.ReactNode;
    icon?: React.ReactNode;
    badge?: React.ReactNode;
  }[];
  className?: string;
}) {
  const layoutGroupId = useId();
  const reduceMotion = useReducedMotion();
  const selectedIndex = Math.max(0, items.findIndex((item) => item.value === value));
  const listRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    const reveal = () => {
      const active = list.querySelector<HTMLElement>(
        '[role="tab"][aria-selected="true"]',
      );
      if (!active) return;
      const item = active.getBoundingClientRect(),
        viewport = list.getBoundingClientRect();
      if (item.left < viewport.left)
        list.scrollLeft += item.left - viewport.left;
      else if (item.right > viewport.right)
        list.scrollLeft += item.right - viewport.right;
    };
    reveal();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(reveal);
    observer.observe(list);
    return () => observer.disconnect();
  }, [value, items.length]);

  return (
    <LayoutGroup id={layoutGroupId}>
      <div
        ref={listRef}
        role="tablist"
        className={cn(
          "axo-tabs-scrollbar relative w-full min-w-0 overflow-x-auto overflow-y-hidden overscroll-x-contain",
          "border-b border-[var(--axo-border)] touch-pan-x",
          className,
        )}
      >
        <div className="relative inline-flex min-w-max items-center gap-1">
          {items.map((it, index) => {
            const active = it.value === value;
            return (
              <button
                key={it.value}
                type="button"
                role="tab"
                aria-selected={active}
                tabIndex={index === selectedIndex ? 0 : -1}
                onClick={() => onChange?.(it.value)}
                onKeyDown={(e) => {
                  if (
                    !onChange ||
                    !["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)
                  )
                    return;
                  e.preventDefault();
                  const next =
                    e.key === "Home"
                      ? 0
                      : e.key === "End"
                        ? items.length - 1
                        : (index +
                            (e.key === "ArrowRight" ? 1 : -1) +
                            items.length) %
                          items.length;
                  listRef.current
                    ?.querySelectorAll<HTMLButtonElement>('[role="tab"]')
                    [next]?.focus();
                  onChange(items[next].value);
                }}
                className={cn(
                  "axo-focus-ring relative inline-flex min-h-11 items-center gap-2 px-3 py-2.5 text-[13px] font-medium whitespace-nowrap cursor-pointer transition-colors duration-200",
                  active
                    ? "text-[var(--axo-text)]"
                    : "text-[var(--axo-text-muted)] hover:text-[var(--axo-text)]",
                )}
              >
                {it.icon}
                <span>{it.label}</span>
                {it.badge !== undefined && it.badge !== null && (
                  <span className="inline-flex min-w-5 shrink-0 items-center justify-center rounded-md bg-[var(--axo-surface-2)] px-1.5 py-0.5 text-[11px] leading-4 tabular-nums text-[var(--axo-text-muted)]">
                    {it.badge}
                  </span>
                )}
                {active && (
                  <motion.span
                    layoutId="active-underline"
                    transition={
                      reduceMotion
                        ? { duration: 0 }
                        : { duration: 0.22, ease: [0.16, 1, 0.3, 1] }
                    }
                    className="absolute left-2 right-2 -bottom-px h-[2px] bg-[var(--axo-accent)] rounded-full"
                  />
                )}
              </button>
            );
          })}
        </div>
      </div>
    </LayoutGroup>
  );
}

/* ────────── Tooltip (light, no portal) ────────── */
export function Tooltip({
  content,
  children,
  side = "top",
}: {
  content: React.ReactNode;
  children: React.ReactNode;
  side?: "top" | "bottom" | "left" | "right";
}) {
  const pos = {
    top: "bottom-full left-1/2 -translate-x-1/2 mb-2",
    bottom: "top-full left-1/2 -translate-x-1/2 mt-2",
    left: "right-full top-1/2 -translate-y-1/2 mr-2",
    right: "left-full top-1/2 -translate-y-1/2 ml-2",
  }[side];
  return (
    <span className="relative inline-flex group">
      {children}
      <span
        className={cn(
          "pointer-events-none absolute z-50 opacity-0 group-hover:opacity-100 transition-opacity duration-200",
          "px-2 py-1 text-[11px] font-medium rounded-[8px] whitespace-nowrap",
          "bg-[var(--axo-surface-inverse)] text-[var(--axo-text-inverse)] shadow-lg",
          pos,
        )}
      >
        {content}
      </span>
    </span>
  );
}

/* ────────── EmptyState ────────── */
export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon?: React.ReactNode;
  title: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35, ease: [0.16, 1, 0.3, 1] }}
      className={cn(
        "flex flex-col items-center justify-center text-center px-6 py-12 rounded-[var(--axo-radius-xl)]",
        "bg-[var(--axo-surface-2)] border border-dashed border-[var(--axo-border-strong)]",
        className,
      )}
    >
      {icon && (
        <div className="w-12 h-12 rounded-[16px] flex items-center justify-center bg-[var(--axo-surface)] border border-[var(--axo-border)] text-[var(--axo-text-muted)] mb-4">
          {icon}
        </div>
      )}
      <div className="text-[15px] font-semibold text-[var(--axo-text)] mb-1">
        {title}
      </div>
      {description && (
        <div className="text-[13px] text-[var(--axo-text-muted)] max-w-md leading-relaxed mb-4">
          {description}
        </div>
      )}
      {action}
    </motion.div>
  );
}

/* ────────── PageHeader ────────── */
export function PageHeader({
  title,
  description,
  actions,
  breadcrumbs,
  meta,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  breadcrumbs?: { label: React.ReactNode; href?: string }[];
  meta?: React.ReactNode;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: -4 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="flex items-start justify-between gap-4 lg:gap-6 mb-5 lg:mb-6 flex-wrap"
    >
      {/* basis-[320px] на телефоне шире экрана — ниже sm колонка заголовка
          занимает всю строку, и блок действий переносится под неё. */}
      <div className="min-w-0 flex-1 basis-full sm:basis-[320px]">
        {breadcrumbs && breadcrumbs.length > 0 && (
          <div className="flex items-center gap-1.5 text-[12px] text-[var(--axo-text-muted)] mb-2">
            {breadcrumbs.map((b, i) => (
              <span key={i} className="inline-flex items-center gap-1.5">
                {i > 0 && (
                  <span className="text-[var(--axo-text-faint)]">/</span>
                )}
                {b.href ? (
                  <Link
                    href={b.href}
                    className="hover:text-[var(--axo-text)] transition-colors"
                  >
                    {b.label}
                  </Link>
                ) : (
                  <span>{b.label}</span>
                )}
              </span>
            ))}
          </div>
        )}
        <h1 className="text-[24px] font-semibold tracking-tight text-[var(--axo-text)] leading-tight break-words">
          {title}
        </h1>
        {description && (
          <p className="text-[14px] text-[var(--axo-text-muted)] mt-1.5 max-w-2xl leading-relaxed">
            {description}
          </p>
        )}
        {meta && <div className="mt-3">{meta}</div>}
      </div>
      {/* shrink-0 держал ряд кнопок в его max-content ширину, и на 360px он
          уезжал за край экрана целиком — flex-wrap внутри не помогал, потому
          что сжиматься было запрещено самому ряду. */}
      {/* Ниже sm кнопки шапки идут колонкой во всю ширину. Своей шириной они
          вставали по две в ряд с переносом, и ряд выглядел случайным: одна
          кнопка сверху, вторая под ней с отступом слева. Селектор через
          потомка нужен из-за EAgentTrigger — он оборачивает свою кнопку в
          собственный div. */}
      {/* :not(.hidden) обязателен: без него правило поднимало display даже у
          блоков, спрятанных через `hidden md:flex`, и они проявлялись на
          телефоне развалившимися в колонку. */}
      {actions && (
        <div className="flex w-full min-w-0 flex-col items-stretch gap-2 sm:flex-row sm:flex-wrap sm:items-center lg:w-auto lg:shrink-0 max-sm:[&>a]:flex max-sm:[&>a]:flex-col max-sm:[&>a]:items-stretch max-sm:[&>div:not(.hidden)]:flex max-sm:[&>div:not(.hidden)]:flex-col max-sm:[&>div:not(.hidden)]:items-stretch max-sm:[&>div:not(.hidden)]:gap-2">
          {actions}
        </div>
      )}
    </motion.div>
  );
}

/* ────────── Skeleton ────────── */
export function Skeleton({
  className,
  style,
}: {
  className?: string;
  style?: React.CSSProperties;
}) {
  return <div className={cn("axo-skeleton", className)} style={style} />;
}

/* ────────── KBD ────────── */
export function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-flex items-center px-1.5 py-0.5 text-[10px] font-medium font-mono rounded-[6px] bg-[var(--axo-surface-2)] border border-[var(--axo-border)] text-[var(--axo-text-muted)]">
      {children}
    </kbd>
  );
}
