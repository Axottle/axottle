"use client";

import { forwardRef, type HTMLAttributes } from "react";
import { motion, type HTMLMotionProps } from "framer-motion";
import { cn } from "./utils";

type Variant = "default" | "elevated" | "flat" | "inverse" | "accent";

export interface CardProps extends HTMLMotionProps<"div"> {
  variant?: Variant;
  hoverable?: boolean;
  padding?: "none" | "sm" | "md" | "lg";
}

const variants: Record<Variant, string> = {
  default:
    "bg-[var(--axo-surface)] border border-[var(--axo-border)] shadow-[var(--axo-shadow-sm)]",
  elevated:
    "bg-[var(--axo-surface)] border border-[var(--axo-border)] shadow-[var(--axo-shadow-md)]",
  flat:
    "bg-[var(--axo-surface-2)] border border-[var(--axo-border)]",
  inverse:
    "bg-[var(--axo-surface-inverse)] text-[var(--axo-text-inverse)] border border-transparent shadow-[var(--axo-shadow-md)]",
  accent:
    "bg-[var(--axo-mint-soft)] border border-[var(--axo-mint)] text-[var(--axo-text)]",
};

const paddings = {
  none: "",
  sm: "p-4",
  md: "p-5",
  lg: "p-6",
};

export const Card = forwardRef<HTMLDivElement, CardProps>(function Card(
  { className, variant = "default", hoverable, padding = "md", children, ...rest },
  ref,
) {
  return (
    <motion.div
      ref={ref}
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35, ease: [0.16, 1, 0.3, 1] }}
      whileHover={hoverable ? { y: -2 } : undefined}
      className={cn(
        "rounded-[var(--axo-radius-xl)] transition-shadow duration-200",
        variants[variant],
        paddings[padding],
        hoverable && "cursor-pointer hover:shadow-[var(--axo-shadow-lg)]",
        className,
      )}
      {...rest}
    >
      {children}
    </motion.div>
  );
});

export function CardHeader({
  title,
  description,
  action,
  className,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex items-start justify-between gap-4 mb-4", className)}>
      <div className="min-w-0">
        <div className="text-[15px] font-semibold tracking-tight text-[var(--axo-text)] leading-tight">
          {title}
        </div>
        {description && (
          <div className="text-[13px] text-[var(--axo-text-muted)] mt-1 leading-relaxed">
            {description}
          </div>
        )}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}

export function CardSection({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("py-3 border-t border-[var(--axo-border)] first:border-t-0 first:pt-0 last:pb-0", className)}>
      {children}
    </div>
  );
}
