if (
    // only forward amounts larger than 100 sat
    HtlcForward.Event.OutgoingAmountMsat >= 100000
) { true } else { false }

