export const DOCUMENT_BACKGROUNDS = {
  coral: { light: "#F8F6F6", dark: "#100D0F" },
  mint: { light: "#F1F6F4", dark: "#07090C" },
} as const;

export function documentBackground(
  mode: "light" | "dark",
  palette: string | null,
) {
  return DOCUMENT_BACKGROUNDS[palette === "mint" ? "mint" : "coral"][mode];
}
