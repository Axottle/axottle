"use client";

import { useEffect, useState } from "react";
import { Star } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Dialog } from "./dialog";

const repository = "https://github.com/Axottle/axottle";

export function RepositoryButton() {
  const { lang } = useI18n();
  const [open, setOpen] = useState(false);
  const english = lang === "en";
  const label = english ? "Axottle on GitHub" : "Axottle на GitHub";

  useEffect(() => {
    if (!open) return;
    const timer = window.setTimeout(() => setOpen(false), 2500);
    return () => window.clearTimeout(timer);
  }, [open]);

  return (
    <>
      <a
        href={repository}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={label}
        title={label}
        onClick={() => setOpen(true)}
        className="axo-focus-ring flex h-10 w-10 shrink-0 items-center justify-center rounded-[12px] border border-[var(--axo-border)] bg-[var(--axo-surface)] text-[var(--axo-text)] transition-colors hover:bg-[var(--axo-surface-2)]"
      >
        <svg className="h-[18px] w-[18px]" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M12 .75a11.25 11.25 0 0 0-3.56 21.92c.56.1.77-.24.77-.54v-2.1c-3.13.68-3.79-1.33-3.79-1.33-.51-1.3-1.25-1.65-1.25-1.65-1.02-.7.08-.68.08-.68 1.13.08 1.72 1.16 1.72 1.16 1 1.72 2.63 1.22 3.27.94.1-.73.39-1.22.71-1.5-2.5-.29-5.13-1.25-5.13-5.57 0-1.23.44-2.23 1.16-3.02-.12-.29-.5-1.43.11-2.98 0 0 .94-.3 3.1 1.15a10.8 10.8 0 0 1 5.62 0c2.15-1.45 3.1-1.15 3.1-1.15.6 1.55.22 2.69.1 2.98.73.8 1.16 1.8 1.16 3.02 0 4.33-2.64 5.28-5.15 5.56.4.35.76 1.03.76 2.08v3.09c0 .3.2.65.78.54A11.25 11.25 0 0 0 12 .75Z" />
        </svg>
      </a>
      <Dialog
        open={open}
        onClose={() => setOpen(false)}
        size="sm"
        ariaLabel={english ? "Support Axottle" : "Поддержите Axottle"}
      >
        <div className="flex flex-col items-center gap-4 py-5 text-center">
          <span className="flex h-14 w-14 items-center justify-center rounded-[18px] bg-[var(--axo-accent)]/12 text-[var(--axo-accent)]">
            <Star className="h-7 w-7" aria-hidden="true" />
          </span>
          <h2 className="text-xl font-semibold text-[var(--axo-text)]">
            {english ? "A star for Axottle?" : "Звёздочку для Axottle?"}
          </h2>
          <p className="max-w-xs text-sm leading-relaxed text-[var(--axo-text-muted)]">
            {english
              ? "If Axottle helps you, support the project with a star on GitHub. Thank you!"
              : "Если Axottle вам помогает, поддержите проект звездой на GitHub. Спасибо!"}
          </p>
        </div>
      </Dialog>
    </>
  );
}
