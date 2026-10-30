"use client";

import { ArrowRight, Check } from "lucide-react";
import { cn } from "./utils";

export function WizardChoice({ selected, title, hint, icon, onClick, disabled = false }: {
  selected: boolean; title: string; hint: string; icon: React.ReactNode; onClick: () => void; disabled?: boolean;
}) {
  return <button type="button" aria-pressed={selected} disabled={disabled} onClick={onClick}
    className={cn("axo-focus-ring flex w-full cursor-pointer items-start gap-3 rounded-[16px] border p-4 text-left transition-colors duration-200 motion-reduce:transition-none disabled:cursor-not-allowed disabled:opacity-60", selected ? "border-[var(--axo-accent)] bg-[var(--axo-surface-2)]" : "border-[var(--axo-border)] hover:bg-[var(--axo-surface-2)]")}>
    <span className="mt-0.5 text-[var(--axo-accent)]">{icon}</span>
    <span className="min-w-0 flex-1"><span className="block text-sm font-semibold text-[var(--axo-text)]">{title}</span><span className="mt-1 block text-[13px] leading-relaxed text-[var(--axo-text-muted)]">{hint}</span></span>
    {selected ? <Check aria-hidden className="mt-1 h-4 w-4 shrink-0 text-[var(--axo-accent)]" /> : <ArrowRight aria-hidden className="mt-1 h-4 w-4 shrink-0 text-[var(--axo-text-faint)]" />}
  </button>;
}
