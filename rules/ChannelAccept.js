if (
    ChannelAccept.Event.FundingAmt >= 750000 && 
    ChannelAccept.OneMl.LastUpdate > 1661227900 &&
    ChannelAccept.OneMl.Noderank.Availability > 100 &&
    ChannelAccept.Amboss.Socials.Info.Email
    // ( 
    //     ChannelAccept.Amboss.Socials.Info.Email.length > 0 ||
    //     ChannelAccept.Amboss.Socials.Info.Twitter.length >0 ||
    //     ChannelAccept.Amboss.Socials.Info.Telegram.length >0 
    // ) 
    // ChannelAccept.Amboss.Amboss.IsPrime == false
) { true } else { false }
