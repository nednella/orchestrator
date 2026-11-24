// The text sizes form a ladder in steps of √1.2 (about 9.5%), from about 0.48 to 2.07.
const STEPS = 8
const STEP_LOG = Math.log(1.2) / 2

const clampStep = (step: number) => Math.min(Math.max(step, -STEPS), STEPS)

// Four decimals keep the saved value short; snapScale maps it back to its step.
const scaleAt = (step: number) => Math.round(1.2 ** (clampStep(step) / 2) * 10000) / 10000

// A scale that is not a positive number is taken as the default.
const stepOf = (scale: number) => (Number.isFinite(scale) && scale > 0 ? Math.round(Math.log(scale) / STEP_LOG) : 0)

export const snapScale = (scale: number) => scaleAt(stepOf(scale))

export const zoomedScale = (scale: number, change: -1 | 0 | 1) => (change === 0 ? 1 : scaleAt(stepOf(scale) + change))
