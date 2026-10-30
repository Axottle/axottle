"use client";

import { useEffect, useRef, useState } from "react";
import { uiAsset } from "@/lib/ui-assets";
import preview from "@/lib/generated/login-preview.json";
import styles from "./login.module.css";

export function LoginBackground() {
  const imageRef = useRef<HTMLImageElement>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const image = imageRef.current;
    if (!image) return;
    let cancelled = false;
    const reveal = async () => {
      if (!image.naturalWidth) return;
      try { await image.decode(); } catch { return; }
      if (!cancelled) setReady(true);
    };
    // A cached image may finish before hydration attaches the load listener.
    if (image.complete) void reveal();
    image.addEventListener("load", reveal);
    return () => { cancelled = true; image.removeEventListener("load", reveal); };
  }, []);

  return <>
    <picture>
      <source media="(max-width: 767px)" srcSet={preview.mobileSrc} />
      <img className={`${styles.background} ${styles.backgroundPreview}`}
        src={preview.src} alt="" aria-hidden="true" draggable={false} />
    </picture>
    <picture>
      <source media="(max-width: 767px)" srcSet={uiAsset("/assets/auth/conservatory-mobile-v2.png")} />
      <img ref={imageRef}
      className={`${styles.background} ${styles.backgroundFull} ${ready ? styles.backgroundReady : ""}`}
      src={uiAsset("/assets/auth/conservatory.png")} alt="" aria-hidden="true"
      fetchPriority="high" decoding="async" draggable={false} />
    </picture>
  </>;
}
