import assets from "@/lib/generated/assets.json";

/** Build-generated, content-versioned URLs; original public paths remain valid. */
export function uiAsset(source: string): string {
  return (assets as Record<string, string>)[source] ?? source;
}

export const mascotAssets = Object.entries(assets)
  .filter(([source]) => source.startsWith("/mascots/") && !source.includes("pixel-") && !source.includes("preview"))
  .map(([, url]) => url);
