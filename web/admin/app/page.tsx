"use client";
import { useEffect } from "react";
export default function Home() {
  useEffect(() => {
    window.location.replace("/new/");
  }, []);
  return <p className="p-8">Axottle</p>;
}
