package biz

import (
	"github.com/google/wire"
)

// BizProviderSet wires the notifier's use cases: healthz, the subscription hub,
// the open-auction registry, the wall clock, the event projector, and the live
// subscription seam.
var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	NewWallClock,
	NewHub,
	NewRegistry,
	NewProjector,

	NewSubscriber,
	wire.Bind(new(UsecaseLive), new(*Subscriber)),
)
