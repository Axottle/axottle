"use client";

import {
  Children,
  Fragment,
  isValidElement,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type CSSProperties,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import { Check, ChevronDown } from "lucide-react";
import { cn } from "./utils";
import { useI18n } from "@/lib/i18n";
import { Input } from "./input";

export interface SelectOption {
  value: string;
  label: React.ReactNode;
  hint?: React.ReactNode;
  icon?: React.ReactNode;
  searchText?: string;
  disabled?: boolean;
}

export interface SelectProps {
  value?: string;
  onChange?: (value: string) => void;
  options: SelectOption[];
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  align?: "start" | "end";
  size?: "sm" | "md";
  invalid?: boolean;
  ariaLabel?: string;
  searchable?: boolean;
  searchPlaceholder?: string;
  searchQueries?: (query: string) => string[];
}

export interface SelectFieldProps {
  value?: string | number;
  onChange?: (event: ChangeEvent<HTMLSelectElement>) => void;
  children?: ReactNode;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  style?: CSSProperties;
  required?: boolean;
  "aria-label"?: string;
}

function collectOptionElements(
  children: ReactNode,
  options: SelectOption[] = [],
): SelectOption[] {
  Children.forEach(children, (child) => {
    if (!isValidElement(child)) return;
    if (child.type === Fragment) {
      collectOptionElements(
        (child.props as { children?: ReactNode }).children,
        options,
      );
      return;
    }
    if (child.type !== "option") return;
    const props = child.props as {
      value?: string | number;
      children?: ReactNode;
      disabled?: boolean;
    };
    options.push({
      value: String(props.value ?? ""),
      label: props.children ?? String(props.value ?? ""),
      disabled: props.disabled,
    });
  });
  return options;
}

/**
 * Compatibility bridge for legacy forms that still express choices as
 * `<option>` children. The rendered control is the same portal-based Select
 * as the rest of the design system; no browser-native dropdown is exposed.
 */
export function SelectField({
  value,
  onChange,
  children,
  placeholder,
  disabled,
  className,
  style,
  "aria-label": ariaLabel,
}: SelectFieldProps) {
  const options = collectOptionElements(children);
  return (
    <div className={className} style={style}>
      <Select
        value={String(value ?? "")}
        onChange={(nextValue) =>
          onChange?.({
            target: { value: nextValue },
          } as ChangeEvent<HTMLSelectElement>)
        }
        options={options}
        placeholder={placeholder}
        disabled={disabled}
        ariaLabel={ariaLabel}
      />
    </div>
  );
}

type MenuPosition = {
  left: number;
  width: number;
  maxHeight: number;
  placement: "top" | "bottom";
  top?: number;
  bottom?: number;
};

export function Select({
  value,
  onChange,
  options,
  placeholder,
  disabled,
  className,
  align = "start",
  size = "md",
  invalid,
  ariaLabel,
  searchable = false,
  searchPlaceholder,
  searchQueries,
}: SelectProps) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [menuPosition, setMenuPosition] = useState<MenuPosition | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const id = useId();
  const resolvedPlaceholder =
    placeholder ?? t("select.placeholder", "Выберите...");
  const searchLabel = searchPlaceholder ?? t("select.search", "Поиск…");
  const queries = searchQueries ? searchQueries(search.trim()) : [search.trim()];
  const filteredOptions = options.filter(
    (option) =>
      !searchable ||
      queries.some((query) => [option.value, option.label, option.hint, option.searchText]
        .filter((part) => typeof part === "string")
        .join(" ")
        .toLocaleLowerCase()
        .includes(query.toLocaleLowerCase())),
  );
  const visibleOptions = searchable
    ? filteredOptions.slice(0, 100)
    : filteredOptions;

  useEffect(() => {
    if (!open) setSearch("");
  }, [open]);

  useLayoutEffect(() => {
    if (!open) return;

    function updatePosition() {
      const trigger = triggerRef.current;
      if (!trigger) return;

      const rect = trigger.getBoundingClientRect();
      const viewportWidth = window.innerWidth;
      const viewportHeight = window.innerHeight;
      const margin = 12;
      const gap = 8;
      const preferredMaxHeight = 280;
      const minUsefulHeight = 160;
      const width = Math.min(rect.width, viewportWidth - margin * 2);
      const rawLeft = align === "end" ? rect.right - width : rect.left;
      const left = Math.min(
        Math.max(margin, rawLeft),
        Math.max(margin, viewportWidth - width - margin),
      );
      const spaceBelow = viewportHeight - rect.bottom - margin - gap;
      const spaceAbove = rect.top - margin - gap;
      const placement =
        spaceBelow < minUsefulHeight && spaceAbove > spaceBelow
          ? "top"
          : "bottom";
      const availableSpace = placement === "top" ? spaceAbove : spaceBelow;
      const maxHeight = Math.min(
        preferredMaxHeight,
        Math.max(120, Math.floor(availableSpace)),
      );

      setMenuPosition({
        left,
        width,
        maxHeight,
        placement,
        ...(placement === "top"
          ? { bottom: viewportHeight - rect.top + gap }
          : { top: rect.bottom + gap }),
      });
    }

    updatePosition();
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [align, open]);

  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (
        !triggerRef.current?.contains(e.target as Node) &&
        !menuRef.current?.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      // The menu owns Escape while open, before an enclosing dialog handles it.
      e.preventDefault();
      e.stopPropagation();
      setOpen(false);
      triggerRef.current?.focus();
    }
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey, true);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey, true);
    };
  }, [open]);

  const current = options.find((o) => o.value === value);
  const mode =
    typeof document !== "undefined"
      ? (document.body.getAttribute("data-axo-mode") ?? "light")
      : "light";
  const menu =
    open && menuPosition && typeof document !== "undefined"
      ? createPortal(
          <AnimatePresence>
            <motion.div
              ref={menuRef}
              data-axo-new
              data-axo-mode={mode}
              initial={{
                opacity: 0,
                y: menuPosition.placement === "top" ? 4 : -4,
                scale: 0.98,
              }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{
                opacity: 0,
                y: menuPosition.placement === "top" ? 4 : -4,
                scale: 0.98,
              }}
              transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
              className={cn(
                // Select is portalled to document.body. It must sit above the
                // highest in-app dialog (route-condition dialog: z-[230]) or
                // an open list appears behind its own modal panel.
                "fixed z-[250] overflow-y-auto p-1.5",
                "bg-[var(--axo-surface)] border border-[var(--axo-border)] rounded-[16px] shadow-[var(--axo-shadow-lg)]",
              )}
              style={{
                left: menuPosition.left,
                width: menuPosition.width,
                maxHeight: menuPosition.maxHeight,
                top: menuPosition.top,
                bottom: menuPosition.bottom,
                transformOrigin:
                  menuPosition.placement === "top"
                    ? "bottom center"
                    : "top center",
              }}
            >
              {searchable && (
                <div className="sticky top-0 z-10 bg-[var(--axo-surface)] pb-1.5">
                  <Input
                    autoFocus
                    type="search"
                    value={search}
                    aria-label={searchLabel}
                    placeholder={searchLabel}
                    onChange={(event) => setSearch(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "ArrowDown") {
                        event.preventDefault();
                        menuRef.current
                          ?.querySelector<HTMLButtonElement>(
                            '[role="option"]:not(:disabled)',
                          )
                          ?.focus();
                      }
                      if (event.key === "Tab") {
                        event.preventDefault();
                        setOpen(false);
                        triggerRef.current?.focus();
                      }
                    }}
                  />
                </div>
              )}
              <div
                role="listbox"
                aria-labelledby={id}
                onKeyDown={(event) => {
                  if (!searchable) return;
                  if (event.key === "Tab") {
                    event.preventDefault();
                    setOpen(false);
                    triggerRef.current?.focus();
                  }
                  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                    event.preventDefault();
                    const items = Array.from(
                      menuRef.current?.querySelectorAll<HTMLButtonElement>(
                        '[role="option"]:not(:disabled)',
                      ) ?? [],
                    );
                    const index = items.indexOf(
                      document.activeElement as HTMLButtonElement,
                    );
                    items[
                      (index +
                        (event.key === "ArrowDown" ? 1 : -1) +
                        items.length) %
                        items.length
                    ]?.focus();
                  }
                }}
              >
                {filteredOptions.length === 0 && (
                  <div className="px-3 py-2 text-[13px] text-[var(--axo-text-muted)]">
                    {search
                      ? t("select.noMatches", "Ничего не найдено")
                      : t("select.noOptions", "Нет вариантов")}
                  </div>
                )}
                {visibleOptions.map((opt) => {
                  const active = opt.value === value;
                  return (
                    <button
                      key={opt.value}
                      type="button"
                      role="option"
                      aria-selected={active}
                      disabled={opt.disabled}
                      onClick={() => {
                        onChange?.(opt.value);
                        setOpen(false);
                        triggerRef.current?.focus();
                      }}
                      className={cn(
                        // Пункт списка на пальце должен быть не ниже 44px —
                        // на мыши это выглядело бы рыхло, поэтому только до sm.
                        "w-full flex items-start gap-2.5 px-2.5 py-2.5 sm:py-2 min-h-[44px] sm:min-h-0 items-center sm:items-start text-left rounded-[10px] text-[13px] cursor-pointer",
                        "transition-colors duration-150 disabled:cursor-not-allowed",
                        active
                          ? "bg-[var(--axo-accent-soft)] text-[var(--axo-accent)]"
                          : "text-[var(--axo-text)] hover:bg-[var(--axo-surface-2)]",
                        opt.disabled && "opacity-50",
                      )}
                    >
                      {opt.icon && (
                        <span className="mt-0.5 shrink-0">{opt.icon}</span>
                      )}
                      <span className="flex-1 min-w-0">
                        <span className="block truncate font-medium">
                          {opt.label}
                        </span>
                        {opt.hint && (
                          <span className="block text-[11px] text-[var(--axo-text-muted)] mt-0.5">
                            {opt.hint}
                          </span>
                        )}
                      </span>
                      {active && <Check className="w-4 h-4 shrink-0 mt-0.5" />}
                    </button>
                  );
                })}
              </div>
              {searchable && filteredOptions.length > visibleOptions.length && (
                <p
                  role="status"
                  className="px-3 py-2 text-xs text-[var(--axo-text-muted)]"
                >
                  {t(
                    "select.refineSearch",
                    "Уточните поиск, чтобы увидеть остальные варианты.",
                  )}
                </p>
              )}
            </motion.div>
          </AnimatePresence>,
          document.body,
        )
      : null;

  return (
    <div className={cn("relative w-full", className)}>
      <button
        ref={triggerRef}
        type="button"
        id={id}
        disabled={disabled}
        onClick={() => setOpen((p) => !p)}
        className={cn(
          "axo-focus-ring w-full inline-flex items-center justify-between gap-2 text-left",
          "cursor-pointer disabled:cursor-not-allowed",
          "bg-[var(--axo-surface)] border border-[var(--axo-border-strong)] hover:border-[var(--axo-text-faint)]",
          "transition-[border-color,box-shadow] duration-200 disabled:opacity-60",
          "rounded-[14px]",
          size === "md"
            ? "h-10 px-3.5 text-sm"
            : "h-10 sm:h-8 px-3 text-[13px]",
          open && "border-[var(--axo-accent)]",
          invalid && "border-[var(--axo-status-crit)]",
        )}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-invalid={invalid || undefined}
        aria-label={ariaLabel}
      >
        <span className="flex items-center gap-2 min-w-0 truncate">
          {current?.icon}
          <span
            className={cn(
              "truncate",
              !current && "text-[var(--axo-text-faint)]",
            )}
          >
            {current?.label ?? resolvedPlaceholder}
          </span>
        </span>
        <ChevronDown
          className={cn(
            "w-4 h-4 text-[var(--axo-text-muted)] shrink-0 transition-transform duration-200",
            open && "rotate-180",
          )}
        />
      </button>

      {menu}
    </div>
  );
}
