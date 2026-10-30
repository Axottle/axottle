"use client";

import { useEffect, useState } from "react";

/** Подписка на медиазапрос. Возвращает false до гидратации: разметка с
 *  сервера рисуется без знания об экране, и любое другое стартовое значение
 *  даёт расхождение гидратации на первом кадре. */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(false);

  useEffect(() => {
    const mql = window.matchMedia(query);
    setMatches(mql.matches);
    const onChange = (e: MediaQueryListEvent) => setMatches(e.matches);
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}

/** Телефон: до 640px — тот же порог, что у брейкпоинта `sm` в Tailwind,
 *  чтобы поведение в JS и в разметке не разъезжалось. */
export function useIsMobile(): boolean {
  return useMediaQuery("(max-width: 639px)");
}

/** Тач-ввод без точного указателя. Отличается от useIsMobile: планшет
 *  широкий, но пальцем по нему попадают так же неточно, как по телефону,
 *  а ноутбук с тачскрином сюда не попадает — у него есть и мышь. */
export function useIsTouch(): boolean {
  return useMediaQuery("(pointer: coarse)");
}
