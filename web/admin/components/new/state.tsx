"use client";

import { AlertTriangle, Loader2, RefreshCw } from "lucide-react";
import { motion } from "framer-motion";
import { cn } from "./utils";
import { Button } from "./button";

import { useI18n } from "@/lib/i18n";

/* Inline loader — для секций внутри карточек */
export function InlineLoader({ label }: { label?: string }) {
  const { t } = useI18n();
  const text = label ?? t("state.loadingData", "Загружаем данные…");
  return (
    <div className="flex items-center justify-center gap-2 py-10 text-[12px] text-[var(--axo-text-muted)]">
      <Loader2 className="w-3.5 h-3.5 animate-spin" />
      {text}
    </div>
  );
}

/* Большая ошибочная карточка — когда вся секция упала */
export function ErrorState({
  title,
  message,
  onRetry,
  className,
}: {
  title?: string;
  message?: string;
  onRetry?: () => void;
  className?: string;
}) {
  const { t } = useI18n();
  const resolvedTitle = title ?? t("state.loadFailedTitle", "Не удалось загрузить");
  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      className={cn(
        "flex items-start gap-3 p-4 rounded-[16px]",
        "bg-[var(--axo-status-crit-soft)] border border-[var(--axo-status-crit-soft)]",
        className,
      )}
    >
      <div className="w-9 h-9 shrink-0 rounded-[12px] bg-[var(--axo-surface)] flex items-center justify-center text-[var(--axo-status-crit)]">
        <AlertTriangle className="w-4 h-4" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-[14px] font-semibold text-[var(--axo-text)]">{resolvedTitle}</div>
        {message && (
          <div className="text-[12px] text-[var(--axo-text-muted)] mt-1 leading-relaxed">{message}</div>
        )}
      </div>
      {onRetry && (
        <Button variant="outline" size="sm" leftIcon={<RefreshCw className="w-3.5 h-3.5" />} onClick={onRetry}>
          {t("common.retry", "Повторить")}
        </Button>
      )}
    </motion.div>
  );
}

/* Маленький бейдж «обновлено N сек назад» — для шапок секций с polling */
export function RefreshedAt({ label, refreshing, onRefresh }: { label: string; refreshing?: boolean; onRefresh?: () => void }) {
  return (
    <button
      type="button"
      onClick={onRefresh}
      className="inline-flex items-center gap-1.5 text-[11px] text-[var(--axo-text-muted)] hover:text-[var(--axo-text)] transition-colors"
    >
      {refreshing ? (
        <Loader2 className="w-3 h-3 animate-spin" />
      ) : (
        <RefreshCw className="w-3 h-3" />
      )}
      {label}
    </button>
  );
}
