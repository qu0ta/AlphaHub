/** Единый логотип AlphaHub: красный знак «a» + название. Используется на всех экранах. */
export function Logo({ compact = false }: { compact?: boolean }) {
  return (
    <span className={`logo ${compact ? "logo--compact" : ""}`} aria-label="AlphaHub">
      <span className="logo__mark" aria-hidden="true">
        a
      </span>
      <span className="logo__name" aria-hidden="true">
        AlphaHub
      </span>
    </span>
  );
}
