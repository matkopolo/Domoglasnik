export function Brand({ onHome }: { onHome?: () => void }) {
  return (
    <button type="button" className="brand" aria-label="DOMOGLASNIK — začetni pogled" onClick={onHome}>
      <span className="brand-mark" aria-hidden="true">
        <svg viewBox="0 0 40 40" focusable="false">
          <path className="brand-house" d="M8.5 18.4 20 8.5l11.5 9.9v12.1a2 2 0 0 1-2 2h-19a2 2 0 0 1-2-2Z" />
          <path className="brand-roof" d="m6.8 19.5 13.2-11 13.2 11" />
          <path className="brand-bars" d="M13 27v-4.2M20 27v-8.1M27 27V14.7" />
          <circle className="brand-point" cx="27" cy="14.7" r="1.8" />
        </svg>
      </span>
      <span className="brand-wordmark"><strong>DOM</strong><span>OGLASNIK</span></span>
    </button>
  );
}
