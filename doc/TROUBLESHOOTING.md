# The signal-cli-rest-api docker container won't start (signal_messenger_signal-cli-rest-api_1 exited with code 0)

If your docker container stops with `signal_messenger_signal-cli-rest-api_1 exited with code 0`, make sure that the host port isn't already occupied by another process (see [here](https://github.com/paprickar/signal-cli-rest-api/issues/2)).

# Sending a message suceeds, but no message is sent

According to [this](https://github.com/AsamK/signal-cli/issues/202) signal-cli ticket here, the receive endpoint needs to be called regularily. So, if sending a message seems to work, but no message is sent, please try to call the [Receive API endpoint](https://bbernhard.github.io/signal-cli-rest-api/#/Messages/get_v1_receive__number_). 

# Cannot send message to group - please first update your profile

If you get this message, it means that you first need to [update your profile](https://bbernhard.github.io/signal-cli-rest-api/#/Profiles/put_v1_profiles__number_) to set a name (and optionally an avatar). 
