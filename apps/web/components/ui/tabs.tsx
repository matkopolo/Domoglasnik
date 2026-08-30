"use client";

import type { KeyboardEvent, ReactNode } from "react";

export interface TabOption<T extends string> { id: T; label: string }

export function Tabs<T extends string>({ label, tabs, active, onChange, children, className = "" }: {
  label: string;
  tabs: TabOption<T>[];
  active: T;
  onChange: (tab: T) => void;
  children: ReactNode;
  className?: string;
}) {
  const handleKey = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    let next = index;
    if (event.key === "ArrowLeft") next = (index - 1 + tabs.length) % tabs.length;
    if (event.key === "ArrowRight") next = (index + 1) % tabs.length;
    if (event.key === "Home") next = 0;
    if (event.key === "End") next = tabs.length - 1;
    onChange(tabs[next].id);
    document.getElementById(`${label}-${tabs[next].id}-tab`)?.focus();
  };
  return <div className={`tabs-layout ${className}`}>
    <div className="tab-list" role="tablist" aria-label={label}>
      {tabs.map((tab, index) => <button key={tab.id} id={`${label}-${tab.id}-tab`}
        type="button" role="tab" aria-selected={active === tab.id}
        aria-controls={`${label}-${tab.id}-panel`} tabIndex={active === tab.id ? 0 : -1}
        className={active === tab.id ? "active" : ""}
        onClick={() => onChange(tab.id)} onKeyDown={(event) => handleKey(event, index)}>
        {tab.label}
      </button>)}
    </div>
    <div role="tabpanel" id={`${label}-${active}-panel`} aria-labelledby={`${label}-${active}-tab`} tabIndex={0}>
      {children}
    </div>
  </div>;
}
