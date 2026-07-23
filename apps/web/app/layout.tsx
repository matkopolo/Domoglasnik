import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Nepremičninski Agent | Pregled trga",
  description: "Analitični pregled prodajnega in najemnega trga nepremičnin v Sloveniji.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="sl">
      <body>{children}</body>
    </html>
  );
}
