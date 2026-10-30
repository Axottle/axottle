import type { Metadata, Viewport } from "next";
import { Providers } from "@/lib/presentation";
import { uiAsset } from "@/lib/ui-assets";
import "./globals.css";
import "@/lib/generated/assets.css";
export const metadata: Metadata = {
  title: "Axottle",
  icons: { icon: uiAsset("/logo.png") },
};
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
  userScalable: true,
};
export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="ru"
      data-axo-mode="light"
      data-mode="light"
      data-style="cloud"
      suppressHydrationWarning
    >
      <head>
        <link rel="stylesheet" href="/assets/demo/fonts.css" />
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){var d=document.documentElement,m='light';try{m=localStorage.getItem('axo-new-mode')||'';}catch(e){}if(location.pathname.indexOf('/login')===0||location.pathname.indexOf('/setup')===0)m='dark';if(m!=='light'&&m!=='dark')m=matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';d.setAttribute('data-axo-mode',m);d.setAttribute('data-mode',m);d.style.colorScheme=m;d.style.backgroundColor=m==='dark'?'#07090C':'#F1F6F4';})();`,
          }}
        />
      </head>
      <body className="antialiased min-h-screen">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
