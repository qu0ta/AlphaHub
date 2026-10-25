import type { RepresentativePosition } from "../types";

/**
 * Поля интерфейса, которых нет в API бэкенда, — хранятся в браузере.
 * Это не данные заявки: бизнес-данные (компания, заявка, финансы, решения)
 * живут только на бэкенде. Здесь — то, что нужно лишь для удобства экрана:
 *  - должность представителя (API её пока не принимает);
 *  - какие ячейки финансов клиент исправил вручную (подсветка в таблице).
 *
 * localStorage может быть недоступен (приватный режим, запрет сайта),
 * поэтому все обращения обёрнуты в try/catch: без него экран просто
 * покажет значения по умолчанию.
 */

const KEY = "alphahub_ui";

interface Representative {
  name: string;
  position: RepresentativePosition;
  positionOther: string;
}

interface Store {
  representatives: Record<string, Representative>; // по id пользователя
  editedCells: Record<string, string[]>; // по id заявки
}

function read(): Store {
  try {
    const parsed = JSON.parse(localStorage.getItem(KEY) ?? "{}") as Partial<Store>;
    return { representatives: parsed.representatives ?? {}, editedCells: parsed.editedCells ?? {} };
  } catch {
    return { representatives: {}, editedCells: {} };
  }
}

function write(store: Store) {
  try {
    localStorage.setItem(KEY, JSON.stringify(store));
  } catch {
    // хранилище недоступно — значение просто не запомнится
  }
}

export const localUi = {
  getRepresentative: (userId: string): Representative | null => read().representatives[userId] ?? null,
  setRepresentative(userId: string, value: Representative) {
    const store = read();
    store.representatives[userId] = value;
    write(store);
  },
  getEditedCells: (applicationId: string): string[] => read().editedCells[applicationId] ?? [],
  setEditedCells(applicationId: string, cells: string[]) {
    const store = read();
    store.editedCells[applicationId] = cells;
    write(store);
  },
};
