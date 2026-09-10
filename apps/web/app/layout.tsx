import type { Metadata } from "next";
import "maplibre-gl/dist/maplibre-gl.css";
import "./globals.css";
import "./redesign.css";
import "./atlas.css";
import "./domoglasnik.css";

export const metadata: Metadata = {
  title: "DOMOGLASNIK | Dejansko sklenjeni nepremičninski posli",
  description: "Analitični pregled prodajnega in najemnega trga nepremičnin v Sloveniji.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="sl">
      <body>{children}</body>
    </html>
  );
}
