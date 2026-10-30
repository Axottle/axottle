"use client";

import {
  forwardRef,
  type InputHTMLAttributes,
  type TextareaHTMLAttributes,
} from "react";
import { cn } from "./utils";

const baseField =
  "axo-input axo-focus-ring w-full [--axo-field-bg:var(--axo-surface)] bg-[var(--axo-field-bg)] border border-[var(--axo-border-strong)] " +
  "text-[var(--axo-text)] placeholder:text-[var(--axo-text-faint)] " +
  "transition-[border-color,box-shadow,background] duration-200 ease-out " +
  "hover:border-[var(--axo-text-faint)] focus:border-[var(--axo-accent)] disabled:opacity-60 " +
  "[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none";

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
  invalid?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, leftIcon, rightIcon, invalid, ...rest },
  ref,
) {
  if (leftIcon || rightIcon) {
    return (
      <div className="relative w-full">
        {leftIcon && (
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--axo-text-muted)] pointer-events-none">
            {leftIcon}
          </span>
        )}
        <input
          ref={ref}
          aria-invalid={invalid || undefined}
          className={cn(
            baseField,
            "h-10 rounded-[14px] text-sm",
            leftIcon ? "pl-10" : "pl-4",
            rightIcon ? "pr-10" : "pr-4",
            invalid && "border-[var(--axo-status-crit)]",
            className,
          )}
          {...rest}
        />
        {rightIcon && (
          <span className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--axo-text-muted)]">
            {rightIcon}
          </span>
        )}
      </div>
    );
  }
  return (
    <input
      ref={ref}
      aria-invalid={invalid || undefined}
      className={cn(
        baseField,
        "h-10 px-4 rounded-[14px] text-sm",
        invalid && "border-[var(--axo-status-crit)]",
        className,
      )}
      {...rest}
    />
  );
});

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  function Textarea({ className, invalid, ...rest }, ref) {
    return (
      <textarea
        ref={ref}
        aria-invalid={invalid || undefined}
        className={cn(
          baseField,
          "min-h-[96px] p-3 rounded-[14px] text-sm resize-y",
          invalid && "border-[var(--axo-status-crit)]",
          className,
        )}
        {...rest}
      />
    );
  },
);

export function FormField({
  label,
  htmlFor,
  hint,
  error,
  required,
  children,
  className,
}: {
  label?: React.ReactNode;
  htmlFor?: string;
  hint?: React.ReactNode;
  error?: React.ReactNode;
  required?: boolean;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      {label && (
        <label
          htmlFor={htmlFor}
          className="text-[13px] font-medium text-[var(--axo-text)]"
        >
          {label}
          {required && (
            <span className="text-[var(--axo-status-crit)] ml-1">*</span>
          )}
        </label>
      )}
      {children}
      {error ? (
        <div role="alert" className="text-[12px] text-[var(--axo-status-crit)]">
          {error}
        </div>
      ) : hint ? (
        <div className="text-[12px] text-[var(--axo-text-muted)] leading-relaxed">
          {hint}
        </div>
      ) : null}
    </div>
  );
}
