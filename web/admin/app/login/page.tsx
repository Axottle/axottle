"use client";
import { useEffect, useState } from "react";
import { ArrowRight, Eye, EyeOff, Lock, UserRound } from "lucide-react";
import { Button } from "@/components/new/button";
import { FormField, Input } from "@/components/new/input";
import { useI18n } from "@/lib/i18n";
import { useText } from "@/lib/state";
import { APIError, request } from "@/lib/api";
import { markAuthed } from "@/lib/session";
import { uiAsset } from "@/lib/ui-assets";
import { LoginBackground } from "./login-background";
import styles from "./login.module.css";
export default function Login() {
  const text = useText(),
    { lang, setLang } = useI18n();
  const [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [code, setCode] = useState(""),
    [totp, setTOTP] = useState(false),
    [show, setShow] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    const abort = new AbortController();
    request<{ setup_complete: boolean }>("setup/status", {
      signal: abort.signal,
    })
      .then((v) => {
        if (!v.setup_complete) window.location.replace("/setup/");
      })
      .catch((err) => {
        if (!abort.signal.aborted)
          setError(err instanceof Error ? err.message : String(err));
      });
    return () => abort.abort();
  }, []);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await request("auth/login", {
        method: "POST",
        body: { username, password, totp_code: totp ? code : undefined },
      });
      markAuthed();
      window.location.replace("/new/");
    } catch (err) {
      if (err instanceof APIError && err.code === "TOTP_REQUIRED") {
        setTOTP(true);
        setCode("");
      } else
        setError(
          text(
            "Не удалось войти. Проверьте данные и повторите попытку.",
            "Could not sign in. Check your credentials and try again.",
          ),
        );
    } finally {
      setBusy(false);
    }
  }
  return (
    <div data-axo-new className={styles.scene}>
      <LoginBackground />
      <header className={styles.header}>
        <div className={styles.brand}>
          <img src={uiAsset("/logo.png")} alt="" />
          <strong className="axo-brand-wordmark">Axottle</strong>
        </div>
        <button
          className={styles.language}
          onClick={() => setLang(lang === "ru" ? "en" : "ru")}
          aria-label={
            lang === "ru" ? "Switch to English" : "Переключить на русский"
          }
        >
          {lang.toUpperCase()}
        </button>
      </header>
      <div className={styles.hero}>
        <h2>
          {text("Всё начинается", "It all starts")}
          <br />
          <em>{text("с вашего проекта", "with your project")}</em>
        </h2>
      </div>
      <main className={styles.main}>
        <section className={styles.card}>
          <div className={styles.heading}>
            <h1>{text("С возвращением", "Welcome back")}</h1>
            <p>{text("Войдите в Axottle", "Sign in to Axottle")}</p>
          </div>
          <form onSubmit={submit} className={styles.form}>
            <FormField
              label={text("Имя пользователя", "Username")}
              htmlFor="username"
            >
              <Input
                id="username"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                leftIcon={<UserRound size={17} />}
                required
                disabled={busy}
              />
            </FormField>
            <FormField label={text("Пароль", "Password")} htmlFor="password">
              <Input
                id="password"
                type={show ? "text" : "password"}
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                leftIcon={<Lock size={17} />}
                required
                disabled={busy}
                rightIcon={
                  <button
                    type="button"
                    onClick={() => setShow(!show)}
                    aria-label={
                      show
                        ? text("Скрыть пароль", "Hide password")
                        : text("Показать пароль", "Show password")
                    }
                  >
                    {show ? <EyeOff size={17} /> : <Eye size={17} />}
                  </button>
                }
              />
            </FormField>
            {totp && (
              <FormField
                label={text("Код из приложения", "Authenticator code")}
                htmlFor="totp"
              >
                <Input
                  id="totp"
                  autoFocus
                  autoComplete="one-time-code"
                  inputMode="numeric"
                  pattern="[0-9]{6}"
                  maxLength={6}
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  required
                  disabled={busy}
                />
              </FormField>
            )}
            {error && (
              <p role="alert" className="text-sm text-[var(--axo-status-crit)]">
                {error}
              </p>
            )}
            <Button
              type="submit"
              loading={busy}
              rightIcon={<ArrowRight size={17} />}
            >
              {text("Войти", "Sign in")}
            </Button>
          </form>
        </section>
      </main>
    </div>
  );
}
