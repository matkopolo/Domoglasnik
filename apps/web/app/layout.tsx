import type { Metadata } from "next";
import "maplibre-gl/dist/maplibre-gl.css";
import "./globals.css";

export const metadata: Metadata = {
  title: "Analitika nepremičnin | Pregled trga",
  description: "Analitični pregled prodajnega in najemnega trga nepremičnin v Sloveniji.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="sl">
      <body>{children}</body>
    </html>
  );
}
