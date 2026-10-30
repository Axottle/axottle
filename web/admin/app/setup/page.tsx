"use client";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/new/button";
import { FormField, Input } from "@/components/new/input";
import {
  WizardDialog,
  WizardSummary,
  type WizardStep,
} from "@/components/new/wizard-dialog";
import { request } from "@/lib/api";
import { useText } from "@/lib/state";
import { uiAsset } from "@/lib/ui-assets";
type Step = "username" | "password" | "review";
export default function Setup() {
  const text = useText(),
    writing = useRef(false);
  const [open, setOpen] = useState(false),
    [step, setStep] = useState<Step>("username"),
    [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [repeat, setRepeat] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    const abort = new AbortController();
    request<{ setup_complete: boolean }>("setup/status", {
      signal: abort.signal,
    })
      .then((v) => {
        if (v.setup_complete) window.location.replace("/login/");
        else if (!abort.signal.aborted) setOpen(true);
      })
      .catch((err) => {
        if (!abort.signal.aborted)
          setError(err instanceof Error ? err.message : String(err));
      });
    return () => abort.abort();
  }, []);
  const steps: WizardStep<Step>[] = [
    {
      id: "username",
      label: text("Владелец", "Owner"),
      heading: text("Как вы будете входить?", "How will you sign in?"),
      hint: text(
        "Создайте единственный защищённый аккаунт владельца Axottle.",
        "Create the protected Axottle owner account.",
      ),
    },
    {
      id: "password",
      label: text("Пароль", "Password"),
      heading: text(
        "Как защитим аккаунт?",
        "How should we protect the account?",
      ),
      hint: text(
        "Используйте от 8 до 72 байт. Двухфакторную защиту можно включить после входа.",
        "Use 8 to 72 bytes. You can enable two-factor protection after signing in.",
      ),
    },
    {
      id: "review",
      label: text("Проверка", "Review"),
      heading: text("Всё готово к созданию?", "Ready to create your account?"),
      hint: text(
        "Владелец управляет доступом команды. Сохраните пароль в надёжном месте.",
        "The owner manages team access. Keep your password somewhere secure.",
      ),
    },
  ];
  async function next() {
    if (writing.current) return;
    setError("");
    if (step === "username") {
      if (!username.trim()) {
        setError(text("Укажите имя пользователя", "Enter a username"));
        return;
      }
      setStep("password");
      return;
    }
    if (step === "password") {
      const bytes = new TextEncoder().encode(password).length;
      if (bytes < 8 || bytes > 72 || password !== repeat) {
        setError(
          text(
            "Проверьте длину пароля и совпадение повторного ввода",
            "Check the password length and confirmation",
          ),
        );
        return;
      }
      setStep("review");
      return;
    }
    writing.current = true;
    setBusy(true);
    try {
      await request("setup/admin", {
        method: "POST",
        body: { username: username.trim(), password },
      });
      setPassword("");
      setRepeat("");
      window.location.replace("/login/");
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      writing.current = false;
      setBusy(false);
    }
  }
  return (
    <div className="grid min-h-dvh place-items-center p-6">
      <div className="text-center">
        <img
          src={uiAsset("/logo.png")}
          alt=""
          className="mx-auto mb-4 h-20 w-20"
        />
        <h1 className="axo-brand-wordmark mb-5">Axottle</h1>
        <Button onClick={() => setOpen(true)}>
          {text("Начать настройку", "Set up Axottle")}
        </Button>
      </div>
      <WizardDialog
        open={open}
        title={text("Первый запуск Axottle", "Welcome to Axottle")}
        steps={steps}
        step={step}
        onStep={(s) => {
          setStep(s);
          setError("");
        }}
        onClose={() => {
          if (!writing.current) setOpen(false);
        }}
        busy={busy}
        footer={
          <>
            <Button
              variant="ghost"
              disabled={busy || step === "username"}
              onClick={() =>
                setStep(step === "review" ? "password" : "username")
              }
            >
              {text("Назад", "Back")}
            </Button>
            <Button loading={busy} onClick={next}>
              {step === "review"
                ? text("Создать владельца", "Create owner")
                : text("Продолжить", "Continue")}
            </Button>
          </>
        }
      >
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void next();
          }}
        >
          {step === "username" && (
            <FormField
              label={text("Имя пользователя", "Username")}
              htmlFor="setup-user"
            >
              <Input
                id="setup-user"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoFocus
                maxLength={120}
                disabled={busy}
              />
            </FormField>
          )}
          {step === "password" && (
            <div className="space-y-4">
              <FormField
                label={text("Пароль", "Password")}
                htmlFor="setup-password"
              >
                <Input
                  id="setup-password"
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={busy}
                />
              </FormField>
              <FormField
                label={text("Повторите пароль", "Confirm password")}
                htmlFor="setup-repeat"
              >
                <Input
                  id="setup-repeat"
                  type="password"
                  autoComplete="new-password"
                  value={repeat}
                  onChange={(e) => setRepeat(e.target.value)}
                  disabled={busy}
                />
              </FormField>
            </div>
          )}
          {step === "review" && (
            <WizardSummary
              rows={[
                {
                  id: "user",
                  label: text("Имя пользователя", "Username"),
                  value: username.trim(),
                },
                {
                  id: "access",
                  label: text("Доступ", "Access"),
                  value: text("Владелец Axottle", "Axottle owner"),
                },
              ]}
            />
          )}
          <button
            type="submit"
            className="sr-only"
            tabIndex={-1}
            aria-hidden
            disabled={busy}
          />
        </form>
        {error && (
          <p
            role="alert"
            className="mt-4 text-sm text-[var(--axo-status-crit)]"
          >
            {error}
          </p>
        )}
      </WizardDialog>
    </div>
  );
}
