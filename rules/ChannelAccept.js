// only channels > 0.75 Mio sats
ChannelAccept.Event.FundingAmt >= 750000 && 
// nodes with high 1ML availability score
ChannelAccept.OneMl.Noderank.Availability > 100 &&
// nodes with a low enough 1ML age rank
ChannelAccept.OneMl.Noderank.Age < 10000 &&
( 
    // only nodes with Amboss contact data
    ChannelAccept.Amboss.Socials.Info.Email ||
    ChannelAccept.Amboss.Socials.Info.Twitter ||
    ChannelAccept.Amboss.Socials.Info.Telegram 
) &&
(
    // elitist: either nodes with amboss prime
    ChannelAccept.Amboss.Amboss.IsPrime ||
    // or nodes with high-ranking capacity
    ChannelAccept.Amboss.GraphInfo.Metrics.CapacityRank < 1000 ||
    // or nodes with high-ranking channel count
    ChannelAccept.Amboss.GraphInfo.Metrics.ChannelsRank < 1000
)&&
(
    // Only allow private channels which are smaller than 10 mio sats
    (ChannelAccept.Event.ChannelFlags & 1) == 0 && 
    ChannelAccept.Event.FundingAmt <= 10000000 ||
    // allow all public channels
    (ChannelAccept.Event.ChannelFlags & 1) == 1
) 


