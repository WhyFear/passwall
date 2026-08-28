export const createRequestGeneration = () => {
  let current = 0;
  return {
    begin: () => ++current,
    invalidate: () => ++current,
    isCurrent: generation => generation === current,
    value: () => current,
  };
};
