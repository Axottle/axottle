"use client";

import { uiAsset } from "@/lib/ui-assets";
import { Fragment, useEffect, useRef, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n";
import { Dialog } from "./dialog";
import { cn } from "./utils";

export type WizardStep<ID extends string = string> = {
  id: ID;
  label: string;
  heading: string;
  hint: string;
};

/** Canonical creation/settings wizard. Domain validation and submission stay
 * with the caller; the shell only permits navigation to completed steps. */
export function WizardDialog<ID extends string>({
  open = true,
  title,
  steps,
  step,
  onStep,
  onClose,
  children,
  footer,
  mascot = uiAsset("/mascots/axolotl-components.png"),
  busy = false,
  size = "xl",
  firstEditableStep = 0,
}: {
  open?: boolean;
  title: string;
  steps: readonly WizardStep<ID>[];
  step: ID;
  onStep: (step: ID) => void;
  onClose: () => void;
  children: ReactNode;
  footer: ReactNode;
  mascot?: string;
  busy?: boolean;
  size?: "xl" | "dashboard";
  firstEditableStep?: number;
}) {
  const { lang } = useI18n();
  const heading = useRef<HTMLHeadingElement>(null);
  const index = steps.findIndex((s) => s.id === step);
  const current = steps[index];
  useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(() => {
      heading.current?.focus({ preventScroll: true });
      heading.current?.scrollIntoView({ block: "nearest" });
    });
    return () => cancelAnimationFrame(frame);
  }, [open, step]);

  return (
    <Dialog
      open={open}
      size={size}
      title={title}
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={footer}
    >
      <div
        data-axo-wizard
        className="grid gap-5 sm:grid-cols-[250px_minmax(0,1fr)] sm:gap-8"
        aria-busy={busy}
      >
        <div className="flex justify-center border-b border-[var(--axo-border)] pb-4 sm:border-b-0 sm:border-r sm:pb-0 sm:pr-7">
          <img
            src={mascot}
            alt=""
            aria-hidden
            draggable={false}
            width={1254}
            height={1254}
            className="pointer-events-none h-[130px] w-[150px] select-none object-contain sm:sticky sm:top-4 sm:mt-16 sm:h-[280px] sm:w-full sm:self-start"
          />
        </div>
        <div className="min-w-0 sm:min-h-[390px]">
          <nav
            aria-label={lang === "ru" ? "Шаги настройки" : "Setup steps"}
            className="mb-4 flex gap-1.5"
          >
            {steps.map((s, n) => (
              <button
                key={s.id}
                type="button"
                disabled={busy || n >= index || n < firstEditableStep}
                aria-current={s.id === step ? "step" : undefined}
                aria-label={`${n + 1}. ${s.label}`}
                onClick={() => onStep(s.id)}
                className="axo-focus-ring flex h-6 min-w-0 flex-1 items-center rounded-full disabled:cursor-default"
              >
                <span
                  className={cn(
                    "h-1 w-full rounded-full",
                    n <= index
                      ? "bg-[var(--axo-accent)]"
                      : "bg-[var(--axo-border)]",
                  )}
                />
              </button>
            ))}
          </nav>
          <p className="text-[12px] font-medium text-[var(--axo-text-muted)]">
            {lang === "ru"
              ? `Шаг ${index + 1} из ${steps.length}`
              : `Step ${index + 1} of ${steps.length}`}
          </p>
          <h2
            ref={heading}
            tabIndex={-1}
            className="axo-focus-ring mt-2 rounded-md text-[21px] font-semibold leading-snug tracking-tight text-[var(--axo-text)]"
          >
            {current?.heading}
          </h2>
          <p className="mt-2 text-[13px] leading-relaxed text-[var(--axo-text-muted)]">
            {current?.hint}
          </p>
          <div className="mt-5 min-w-0 space-y-4">{children}</div>
        </div>
      </div>
    </Dialog>
  );
}

export type WizardSummaryRow = {
  id: string;
  label: string;
  value: ReactNode;
  hint?: ReactNode;
};

/** Shared read-only rows for known data and the final review. */
export function WizardSummary({ rows }: { rows: readonly WizardSummaryRow[] }) {
  return (
    <dl className="grid min-w-0 grid-cols-1 gap-x-5 gap-y-1 rounded-2xl bg-[var(--axo-surface-2)] p-4 text-[13px] sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] sm:gap-y-4 sm:p-5">
      {rows.map((row) => (
        <Fragment key={row.id}>
          <dt className="min-w-0 break-words text-[var(--axo-text-muted)]">
            {row.label}
          </dt>
          <dd className="mb-3 min-w-0 break-words font-medium sm:mb-0">
            <span className="block">{row.value}</span>
            {row.hint && (
              <span className="mt-1 block text-xs font-normal text-[var(--axo-text-muted)]">
                {row.hint}
              </span>
            )}
          </dd>
        </Fragment>
      ))}
    </dl>
  );
}
