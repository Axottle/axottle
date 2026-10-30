"use client";
import { useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { ShieldCheck } from "lucide-react";
import { Button } from "@/components/new/button";
import { Card, CardHeader } from "@/components/new/card";
import { FormField, Input } from "@/components/new/input";
import { PageHeader } from "@/components/new/feedback";
import { ErrorState, InlineLoader } from "@/components/new/state";
import { useToast } from "@/components/new/toast";
import { request } from "@/lib/api";
import { useResource, useSession, useText } from "@/lib/state";
export default function Profile() {
  const text = useText(),
    { me } = useSession(),
    toast = useToast(),
    status = useResource<{ enabled: boolean; pending: boolean }>("auth/totp");
  const [old, setOld] = useState(""),
    [password, setPassword] = useState(""),
    [repeat, setRepeat] = useState(""),
    [code, setCode] = useState(""),
    [confirmPassword, setConfirmPassword] = useState(""),
    [setup, setSetup] = useState<{ secret: string; uri: string } | null>(null),
    [passwordBusy, setPasswordBusy] = useState(false),
    [mfaBusy, setMFABusy] = useState(false),
    [error, setError] = useState("");
  async function change(e: React.FormEvent) {
    e.preventDefault();
    if (passwordBusy) return;
    setError("");
    const n = new TextEncoder().encode(password).length;
    if (n < 8 || n > 72 || password !== repeat) {
      setError(
        text(
          "Проверьте длину нового пароля и повторный ввод",
          "Check the new password length and confirmation",
        ),
      );
      return;
    }
    setPasswordBusy(true);
    try {
      await request("auth/change-password", {
        method: "POST",
        body: { old_password: old, new_password: password },
      });
      setOld("");
      setPassword("");
      setRepeat("");
      toast.success(text("Пароль изменён", "Password changed"));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setPasswordBusy(false);
    }
  }
  async function mfa(action: "setup" | "confirm" | "disable") {
    if (mfaBusy) return;
    setMFABusy(true);
    setError("");
    try {
      if (action === "setup") {
        setSetup(await request("auth/totp/setup", { method: "POST" }));
        setCode("");
      } else {
        await request(
          action === "disable" ? "auth/totp" : "auth/totp/confirm",
          {
            method: action === "disable" ? "DELETE" : "POST",
            body: {
              ...(action === "disable" ? { password: confirmPassword } : {}),
              code,
            },
          },
        );
        setSetup(null);
        setCode("");
        setConfirmPassword("");
        status.refresh();
        toast.success(
          text("Настройки защиты сохранены", "Security settings saved"),
        );
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setMFABusy(false);
    }
  }
  return (
    <>
      <PageHeader
        title={text("Мой аккаунт", "My account")}
        description={me?.display_name || me?.username}
      />
      {error && (
        <p role="alert" className="mb-5 text-sm text-[var(--axo-status-crit)]">
          {error}
        </p>
      )}
      <div className="grid gap-5 lg:grid-cols-2">
        <Card>
          <CardHeader title={text("Сменить пароль", "Change password")} />
          <form onSubmit={change} className="space-y-4">
            <FormField
              label={text("Текущий пароль", "Current password")}
              htmlFor="old-password"
            >
              <Input
                id="old-password"
                type="password"
                autoComplete="current-password"
                value={old}
                onChange={(e) => setOld(e.target.value)}
                required
                disabled={passwordBusy}
              />
            </FormField>
            <FormField
              label={text("Новый пароль", "New password")}
              htmlFor="new-password"
            >
              <Input
                id="new-password"
                type="password"
                autoComplete="new-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                disabled={passwordBusy}
              />
            </FormField>
            <FormField
              label={text("Повторите пароль", "Confirm password")}
              htmlFor="repeat-password"
            >
              <Input
                id="repeat-password"
                type="password"
                autoComplete="new-password"
                value={repeat}
                onChange={(e) => setRepeat(e.target.value)}
                required
                disabled={passwordBusy}
              />
            </FormField>
            <Button type="submit" loading={passwordBusy}>
              {text("Изменить пароль", "Change password")}
            </Button>
          </form>
        </Card>
        <Card>
          <CardHeader
            title={text("Двухфакторная защита", "Two-factor protection")}
            action={<ShieldCheck size={22} />}
          />
          {status.loading ? (
            <InlineLoader />
          ) : status.error ? (
            <ErrorState message={status.error} onRetry={status.refresh} />
          ) : (
            <div className="space-y-4">
              <p className="text-sm text-[var(--axo-text-muted)]">
                {status.data?.enabled
                  ? text(
                      "Защита включена. Для входа требуется код из приложения.",
                      "Protection is enabled. Sign-in requires an authenticator code.",
                    )
                  : text(
                      "Добавьте одноразовый код к вашему паролю.",
                      "Add an authenticator code to your password.",
                    )}
              </p>
              {setup && (
                <>
                  <div className="w-fit rounded-xl bg-white p-4">
                    <QRCodeSVG
                      value={setup.uri}
                      size={180}
                      title={text(
                        "Код для приложения аутентификации",
                        "Authenticator setup code",
                      )}
                    />
                  </div>
                  <FormField
                    label={text(
                      "Ключ для ручной настройки",
                      "Manual setup key",
                    )}
                    htmlFor="totp-secret"
                  >
                    <Input
                      id="totp-secret"
                      readOnly
                      value={setup.secret}
                      className="font-mono"
                    />
                  </FormField>
                </>
              )}
              {status.data?.enabled || setup ? (
                <form
                  className="space-y-4"
                  onSubmit={(e) => {
                    e.preventDefault();
                    void mfa(status.data?.enabled ? "disable" : "confirm");
                  }}
                >
                  {status.data?.enabled && (
                    <FormField
                      label={text("Текущий пароль", "Current password")}
                      htmlFor="disable-password"
                    >
                      <Input
                        id="disable-password"
                        type="password"
                        autoComplete="current-password"
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        required
                        disabled={mfaBusy}
                      />
                    </FormField>
                  )}
                  <FormField
                    label={text("Код из приложения", "Authenticator code")}
                    htmlFor="mfa-code"
                  >
                    <Input
                      id="mfa-code"
                      autoComplete="one-time-code"
                      inputMode="numeric"
                      pattern="[0-9]{6}"
                      maxLength={6}
                      value={code}
                      onChange={(e) => setCode(e.target.value)}
                      required
                      disabled={mfaBusy}
                    />
                  </FormField>
                  <Button
                    type="submit"
                    variant={status.data?.enabled ? "danger" : "primary"}
                    loading={mfaBusy}
                  >
                    {status.data?.enabled
                      ? text("Отключить защиту", "Disable protection")
                      : text("Подтвердить и включить", "Confirm and enable")}
                  </Button>
                </form>
              ) : (
                <Button onClick={() => mfa("setup")} loading={mfaBusy}>
                  {text("Настроить защиту", "Set up protection")}
                </Button>
              )}
            </div>
          )}
        </Card>
      </div>
    </>
  );
}
