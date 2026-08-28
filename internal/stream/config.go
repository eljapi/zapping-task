package stream

import "time"

const SupportedVersion = 3

/*
How often the live window advances one segment. Ten seconds because that is
what the source material declares, and it is deliberately not configurable: a
wrong value here does not fail, it breaks playback in silence.
*/
const TickInterval = 10 * time.Second
