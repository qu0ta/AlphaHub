import type { HistoryEvent } from "../../types";

/** «Ход заявки» — история событий с маркерами. */
export function Timeline({ events }: { events: Array<Pick<HistoryEvent, "id" | "title" | "text" | "tone">> }) {
  return (
    <ol className="timeline">
      {events.map((event, index) => (
        <li key={event.id} className={`timeline__item timeline__item--${event.tone}`}>
          <span className="timeline__rail" aria-hidden="true">
            <span className="timeline__marker" />
            {index < events.length - 1 && <span className="timeline__line" />}
          </span>
          <div className="timeline__body">
            <p className="timeline__title">{event.title}</p>
            {event.text && <p className="timeline__text">{event.text}</p>}
          </div>
        </li>
      ))}
    </ol>
  );
}
