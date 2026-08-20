const MAX_CONCURRENT = 4;

let active = 0;
const waiters: Array<() => void> = [];

function drain() {
  while (active < MAX_CONCURRENT && waiters.length > 0) {
    active++;
    waiters.shift()?.();
  }
}

/** Limit parallel candle snapshot fetches for list sparklines. */
export function runSparklineFetch<T>(fn: () => Promise<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    const run = () => {
      fn()
        .then(resolve, reject)
        .finally(() => {
          active--;
          drain();
        });
    };
    waiters.push(run);
    drain();
  });
}
