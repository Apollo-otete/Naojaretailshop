interface QuantityControlProps {
  quantity: number;
  onIncrease: () => void;
  onDecrease: () => void;
  size?: 'sm' | 'md';
}

export default function QuantityControl({
  quantity,
  onIncrease,
  onDecrease,
  size = 'md',
}: QuantityControlProps) {
  const btnSize = size === 'sm' ? 'w-8 h-8' : 'w-10 h-10';
  const textSize = size === 'sm' ? 'text-sm' : 'text-base';

  return (
    <div className="flex items-center gap-2.5">
      <button
        onClick={onDecrease}
        className={`${btnSize} rounded-control border border-gray-200 bg-white hover:bg-surface transition-colors flex items-center justify-center`}
        aria-label="Decrease quantity"
      >
        -
      </button>
      <span className={`font-bold ${textSize}`}>{quantity}</span>
      <button
        onClick={onIncrease}
        className={`${btnSize} rounded-control border border-gray-200 bg-white hover:bg-surface transition-colors flex items-center justify-center`}
        aria-label="Increase quantity"
      >
        +
      </button>
    </div>
  );
}
