"use client";
import { useEffect, useRef, useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { Button } from "@/components/new/button";
import { FormField, Input } from "@/components/new/input";
import { WizardDialog, WizardSummary, type WizardStep } from "@/components/new/wizard-dialog";
import { request } from "@/lib/api";
import { useText } from "@/lib/state";

type Step = "password" | "app" | "review";
export function TOTPWizard({ open, onClose, onEnabled }: { open: boolean; onClose: () => void; onEnabled: () => void }) {
  const text = useText(), writing = useRef(false);
  const [step,setStep] = useState<Step>("password"), [password,setPassword] = useState(""), [code,setCode] = useState("");
  const [setup,setSetup] = useState<{secret:string;uri:string}|null>(null), [busy,setBusy] = useState(false), [error,setError] = useState("");
  useEffect(() => { setStep("password"); setPassword(""); setCode(""); setSetup(null); setError(""); },[open]);
  const steps: WizardStep<Step>[] = [
    {id:"password",label:text("Пароль","Password"),heading:text("Подтвердите, что это вы","Confirm your identity"),hint:text("Введите текущий пароль перед подключением 2FA.","Enter your current password before enrolling 2FA.")},
    {id:"app",label:text("Приложение","Authenticator"),heading:text("Добавьте аккаунт в приложение","Add the account to your authenticator"),hint:text("Отсканируйте QR-код и введите код из приложения.","Scan the QR code and enter the authenticator code.")},
    {id:"review",label:text("Проверка","Review"),heading:text("Включить двухфакторный вход?","Enable two-factor sign-in?"),hint:text("Для входа понадобятся пароль и код из приложения.","Sign-in will require your password and an authenticator code.")},
  ];
  async function next() {
    if(writing.current) return;
    setError("");
    if(step==="app") { if(!/^\d{6}$/.test(code)) { setError(text("Введите шестизначный код","Enter a six-digit code")); return; } setStep("review"); return; }
    if(step==="password" && !password) return;
    writing.current=true; setBusy(true);
    try {
      if(step==="password") { if(!setup) setSetup(await request("auth/totp/setup",{method:"POST",body:{password}})); setStep("app"); }
      else { await request("auth/totp/confirm",{method:"POST",body:{password,code}}); setPassword("");setSetup(null);onEnabled(); }
    } catch(err) { setError(err instanceof Error ? err.message : String(err)); }
    finally { writing.current=false;setBusy(false); }
  }
  return <WizardDialog<Step> open={open} title={text("Двухфакторная защита","Two-factor protection")} steps={steps} step={step} onStep={setStep} onClose={onClose} busy={busy} footer={<>
    <Button variant="secondary" disabled={busy} onClick={() => step==="password" ? onClose() : setStep(step==="review"?"app":"password")}>{step==="password"?text("Отмена","Cancel"):text("Назад","Back")}</Button>
    <Button loading={busy} disabled={busy || (step==="password" && !password)} onClick={() => void next()}>{step==="review"?text("Подтвердить и включить","Confirm and enable"):text("Продолжить","Continue")}</Button>
  </>}>
    {step==="password" && <FormField label={text("Текущий пароль","Current password")} htmlFor="enroll-password"><Input id="enroll-password" type="password" autoComplete="current-password" value={password} onChange={e=>setPassword(e.target.value)} disabled={busy} /></FormField>}
    {step==="app" && setup && <><div className="mx-auto w-fit rounded-xl bg-white p-4"><QRCodeSVG value={setup.uri} size={180} title={text("Код настройки 2FA","2FA setup code")} /></div><FormField label={text("Ключ для ручной настройки","Manual setup key")}><Input readOnly value={setup.secret} /></FormField><FormField label={text("Код из приложения","Authenticator code")} htmlFor="enroll-code"><Input id="enroll-code" inputMode="numeric" autoComplete="one-time-code" value={code} maxLength={6} onChange={e=>setCode(e.target.value.replace(/\D/g,""))} /></FormField></>}
    {step==="review" && <WizardSummary rows={[{id:"method",label:text("Способ входа","Sign-in method"),value:text("Пароль и приложение-аутентификатор","Password and authenticator app")}]} />}
    {error && <p role="alert" className="text-sm text-[var(--axo-status-crit)]">{error}</p>}
  </WizardDialog>;
}
