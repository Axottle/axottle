"use client";

import { useEffect } from "react";

/** Замораживает прокрутку страницы, пока `active`.
 *
 *  `overflow: hidden` на body для этого недостаточно: Safari на iOS его для
 *  документа игнорирует и продолжает тащить страницу под открытым слоем.
 *  Работает только `position: fixed`, но он сбрасывает позицию прокрутки в
 *  начало — поэтому её приходится запоминать, компенсировать отрицательным
 *  `top` и возвращать при снятии блокировки.
 *
 *  Вложенные блокировки считаются: если поверх диалога открылся ещё один,
 *  снятие внутреннего не должно разморозить страницу под внешним. */
let lockCount = 0;
let savedScrollY = 0;

function lock() {
  lockCount += 1;
  if (lockCount > 1) return;
  savedScrollY = window.scrollY;
  const { style } = document.body;
  style.position = "fixed";
  style.top = `-${savedScrollY}px`;
  style.left = "0";
  style.right = "0";
  style.width = "100%";
  style.overflow = "hidden";
}

function unlock() {
  lockCount = Math.max(0, lockCount - 1);
  if (lockCount > 0) return;
  const { style } = document.body;
  style.position = "";
  style.top = "";
  style.left = "";
  style.right = "";
  style.width = "";
  style.overflow = "";
  // instant, а не smooth: анимированный возврат читается как «страницу
  // дёрнуло» ровно в тот момент, когда слой закрывается.
  window.scrollTo({ top: savedScrollY, behavior: "instant" as ScrollBehavior });
}

export function useScrollLock(active: boolean) {
  useEffect(() => {
    if (!active) return;
    lock();
    return unlock;
  }, [active]);
}
