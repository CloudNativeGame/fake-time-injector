-module(gettime).
-export([print_current_time/0]).

print_current_time() ->
    {H, M, S} = erlang:localtime(),
    io:format("The current time is ~2.2.0f:~2.2.0f:~2.2.0f.~n", [H, M, S]).