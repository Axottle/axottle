"use client";

import { useEffect, useId, useRef } from "react";
import { createPortal } from "react-dom";
import {
  AnimatePresence,
  motion,
  useDragControls,
  useReducedMotion,
  type PanInfo,
} from "framer-motion";
import { X } from "lucide-react";
import { cn } from "./utils";
import { useI18n } from "@/lib/i18n";
import { useIsMobile } from "@/lib/new/use-media";
import { useScrollLock } from "@/lib/new/use-scroll-lock";

/** Порог закрытия нижнего листа свайпом: либо утащили достаточно далеко,
 *  либо коротко и резко «смахнули». Без второго условия быстрый флик пальцем
 *  не срабатывает — палец не успевает пройти расстояние. */
export function shouldDismissSheet(info: PanInfo): boolean {
  return info.offset.y > 110 || info.velocity.y > 700;
}

interface DialogProps {
  open: boolean;
  onClose: () => void;
  ariaLabel?: string;
  title?: React.ReactNode;
  description?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
  size?: "sm" | "md" | "lg" | "xl" | "dashboard";
  zIndexClassName?: string;
  bodyClassName?: string;
  panelClassName?: string;
  scrimOpacity?: number;
  /** Allow decorative artwork to cross the desktop panel edge. Keep false for
   * long, scrollable forms. */
  allowOverflow?: boolean;
  /** Place the mobile drag handle over artwork instead of reserving a strip. */
  mobileHandleOverlay?: boolean;
}

const sizes = {
  sm: "max-w-md",
  md: "max-w-xl",
  lg: "max-w-2xl",
  xl: "max-w-4xl",
  dashboard: "max-w-[1180px]",
};

export function Dialog({
  open,
  onClose,
  ariaLabel,
  title,
  description,
  children,
  footer,
  size = "md",
  zIndexClassName = "z-[100]",
  bodyClassName,
  panelClassName,
  scrimOpacity = 0.04,
  allowOverflow = false,
  mobileHandleOverlay = false,
}: DialogProps) {
  const { t } = useI18n();
  const panelRef = useRef<HTMLDivElement>(null);
  const onCloseRef = useRef(onClose);
  const titleId = useId();
  const reduceMotion = useReducedMotion();
  const isMobile = useIsMobile();
  const dragControls = useDragControls();
  useScrollLock(open);
  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);
  useEffect(() => {
    if (!open) return;
    const previouslyFocused =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    const focusableSelector =
      "button:not([disabled]), a[href], input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex='-1'])";
    const frame = requestAnimationFrame(() => {
      const first =
        panelRef.current?.querySelector<HTMLElement>(focusableSelector);
      first?.focus();
    });
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCloseRef.current();
      if (e.key === "Tab" && panelRef.current) {
        const focusable = Array.from(
          panelRef.current.querySelectorAll<HTMLElement>(focusableSelector),
        );
        if (focusable.length === 0) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener("keydown", onKey);
    return () => {
      cancelAnimationFrame(frame);
      document.removeEventListener("keydown", onKey);
      previouslyFocused?.focus();
    };
  }, [open]);

  if (typeof document === "undefined") return null;

  return createPortal(
    <AnimatePresence>
      {open && (
        <div
          className={cn(
            // На телефоне диалог прижат к низу и занимает всю ширину: так он
            // ближе к большому пальцу, а поля по краям съедали бы и без того
            // узкую строку формы.
            "fixed inset-0 flex items-end justify-center p-0 sm:items-center sm:p-4",
            zIndexClassName,
          )}
          data-axo-new
          data-axo-mode={document.body.getAttribute("data-axo-mode") ?? "light"}
          style={{ background: "transparent" }}
        >
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.16 }}
            className="absolute inset-0"
            style={{
              background: `rgba(0, 0, 0, ${scrimOpacity})`,
              backdropFilter: "blur(1.5px) saturate(0.95)",
              WebkitBackdropFilter: "blur(1.5px) saturate(0.95)",
            }}
            onClick={onClose}
          />
          <motion.div
            ref={panelRef}
            role="dialog"
            aria-modal="true"
            aria-label={ariaLabel}
            aria-labelledby={title ? titleId : undefined}
            // Нижний лист выезжает снизу, а не всплывает из центра: жест
            // соответствует тому, откуда он появляется.
            initial={
              reduceMotion
                ? { opacity: 0 }
                : isMobile
                  ? { opacity: 0, y: 24 }
                  : { opacity: 0, scale: 0.96, y: 8 }
            }
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={
              reduceMotion
                ? { opacity: 0 }
                : isMobile
                  ? { opacity: 0, y: 24 }
                  : { opacity: 0, scale: 0.96, y: 8 }
            }
            transition={{
              duration: reduceMotion ? 0.01 : 0.22,
              ease: [0.16, 1, 0.3, 1],
            }}
            // Свайп вниз закрывает лист. dragListener выключен, а жест
            // запускается только с шапки и полоски-ухватки: иначе перетаскивание
            // перехватывало бы вертикальную прокрутку содержимого.
            drag={isMobile && !reduceMotion ? "y" : false}
            dragControls={dragControls}
            dragListener={false}
            dragConstraints={{ top: 0, bottom: 0 }}
            dragElastic={{ top: 0, bottom: 0.8 }}
            onDragEnd={(_, info) => {
              if (shouldDismissSheet(info)) onClose();
            }}
            className={cn(
              "relative w-full bg-[var(--axo-surface)] border border-[var(--axo-border)]",
              allowOverflow ? "overflow-visible" : "overflow-hidden",
              "flex flex-col",
              // dvh: с адресной строкой Safari 90vh уходит под неё вместе с
              // футером, где стоят «Сохранить» и «Отмена».
              "max-h-[92dvh] sm:max-h-[90vh]",
              "rounded-t-[24px] rounded-b-none border-b-0 sm:rounded-[24px] sm:border-b",
              sizes[size],
              panelClassName,
            )}
            style={{
              boxShadow: "var(--axo-shadow-xl), 0 0 0 1px var(--axo-border)",
            }}
          >
            {/* Полоска-ухватка: и подсказка, что перед тобой лист, и рукоятка
                для жеста — перетаскивание начинается отсюда и с шапки. */}
            <div
              className={cn("flex touch-none justify-center pt-2.5 pb-1.5 sm:hidden",
                mobileHandleOverlay && "absolute inset-x-0 top-0 z-10")}
              onPointerDown={(e) => dragControls.start(e)}
            >
              <span className="axo-sheet-grab" />
            </div>
            {title && (
              <div
                onPointerDown={(e) => {
                  if (isMobile) dragControls.start(e);
                }}
                className="shrink-0 touch-none sm:touch-auto px-4 py-4 sm:px-6 sm:py-5 border-b border-[var(--axo-border)] flex items-start justify-between gap-3 sm:gap-4"
              >
                <div className="min-w-0">
                  <div
                    id={titleId}
                    className="text-[16px] font-semibold text-[var(--axo-text)]"
                  >
                    {title}
                  </div>
                  {description && (
                    <div className="text-[13px] text-[var(--axo-text-muted)] mt-1 leading-relaxed">
                      {description}
                    </div>
                  )}
                </div>
                <button
                  type="button"
                  onClick={onClose}
                  aria-label={t("common.close", "Закрыть")}
                  className="axo-focus-ring shrink-0 -mr-1 sm:-mr-2 -mt-1 w-10 h-10 sm:w-9 sm:h-9 rounded-[12px] flex items-center justify-center text-[var(--axo-text-muted)] hover:text-[var(--axo-text)] hover:bg-[var(--axo-surface-2)] transition-colors"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>
            )}
            <div
              className={cn(
                allowOverflow
                  ? "flex-1 overflow-visible p-4 sm:p-6"
                  : "flex-1 overflow-y-auto overscroll-contain p-4 sm:p-6",
                bodyClassName,
              )}
            >
              {children}
            </div>
            {/* Кнопки в подвале на телефоне делят строку поровну: две кнопки
                по половине шире и надёжнее под палец, чем прижатые к правому
                краю по своей ширине. */}
            {footer && (
              <div className="shrink-0 px-4 pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] sm:px-6 sm:py-4 border-t border-[var(--axo-border)] bg-[var(--axo-surface-2)]/40 flex flex-wrap items-center justify-end gap-2 max-sm:[&>button]:flex-1">
                {footer}
              </div>
            )}
          </motion.div>
        </div>
      )}
    </AnimatePresence>,
    document.body,
  );
}

/* ────────── Drawer (с правого края) ────────── */
export function Drawer({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  width = 480,
}: {
  open: boolean;
  onClose: () => void;
  title?: React.ReactNode;
  description?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
  width?: number;
}) {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const reduceMotion = useReducedMotion();
  const dragControls = useDragControls();
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  // Пока панель открыта, страница под ней не прокручивается — иначе на
  // телефоне палец, скроллящий содержимое панели, уводит фон.
  useScrollLock(open);

  if (typeof document === "undefined") return null;

  return createPortal(
    <AnimatePresence>
      {open && (
        <div
          className="fixed inset-0 z-[100]"
          data-axo-new
          data-axo-mode={document.body.getAttribute("data-axo-mode") ?? "light"}
          style={{ background: "transparent" }}
        >
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.16 }}
            className="absolute inset-0"
            style={{
              background: "rgba(0, 0, 0, 0.04)",
              backdropFilter: "blur(1.5px) saturate(0.95)",
              WebkitBackdropFilter: "blur(1.5px) saturate(0.95)",
            }}
            onClick={onClose}
          />
          {/* На телефоне панель выезжает снизу и занимает почти весь экран:
              шторка шириной во весь вьюпорт, приехавшая сбоку, читается как
              смена страницы, а не как временный слой над ней. */}
          <motion.aside
            initial={isMobile ? { y: "100%" } : { x: "100%" }}
            animate={isMobile ? { y: 0 } : { x: 0 }}
            exit={isMobile ? { y: "100%" } : { x: "100%" }}
            transition={{ type: "spring", stiffness: 380, damping: 36 }}
            drag={isMobile && !reduceMotion ? "y" : false}
            dragControls={dragControls}
            dragListener={false}
            dragConstraints={{ top: 0, bottom: 0 }}
            dragElastic={{ top: 0, bottom: 0.8 }}
            onDragEnd={(_, info) => {
              if (shouldDismissSheet(info)) onClose();
            }}
            className={cn(
              "absolute bg-[var(--axo-surface)] shadow-[var(--axo-shadow-xl)] flex flex-col",
              "inset-x-0 bottom-0 max-h-[92dvh] rounded-t-[24px] border-t border-[var(--axo-border)]",
              "sm:inset-x-auto sm:top-0 sm:right-0 sm:h-full sm:max-h-none sm:rounded-none sm:border-t-0 sm:border-l sm:border-[var(--axo-border)]",
            )}
            style={
              isMobile ? undefined : { width: `min(${width}px, 100vw)` }
            }
          >
            <div
              className="flex touch-none justify-center pt-2.5 pb-1.5 sm:hidden"
              onPointerDown={(e) => dragControls.start(e)}
            >
              <span className="axo-sheet-grab" />
            </div>
            {title && (
              <div
                onPointerDown={(e) => {
                  if (isMobile) dragControls.start(e);
                }}
                className="touch-none sm:touch-auto px-4 py-4 sm:px-6 sm:py-5 border-b border-[var(--axo-border)] flex items-start justify-between gap-3 sm:gap-4"
              >
                <div className="min-w-0">
                  <div className="text-[16px] font-semibold text-[var(--axo-text)]">
                    {title}
                  </div>
                  {description && (
                    <div className="text-[13px] text-[var(--axo-text-muted)] mt-1 leading-relaxed">
                      {description}
                    </div>
                  )}
                </div>
                <button
                  type="button"
                  onClick={onClose}
                  aria-label={t("common.close", "Закрыть")}
                  className="axo-focus-ring shrink-0 -mr-1 sm:-mr-2 -mt-1 w-10 h-10 sm:w-9 sm:h-9 rounded-[12px] flex items-center justify-center text-[var(--axo-text-muted)] hover:text-[var(--axo-text)] hover:bg-[var(--axo-surface-2)] transition-colors"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>
            )}
            <div className="flex-1 overflow-y-auto overscroll-contain p-4 sm:p-6">
              {children}
            </div>
            {footer && (
              <div className="px-4 pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] sm:px-6 sm:py-4 border-t border-[var(--axo-border)] bg-[var(--axo-surface-2)]/40 flex flex-wrap items-center justify-end gap-2 max-sm:[&>button]:flex-1">
                {footer}
              </div>
            )}
          </motion.aside>
        </div>
      )}
    </AnimatePresence>,
    document.body,
  );
}
