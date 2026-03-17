export const oneOf = <T>(...values: T[]) => (value: T): boolean => values.includes(value);
