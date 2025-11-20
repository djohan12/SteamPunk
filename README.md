# SteamPunk
The "Go"-to Discord bot to see what all your friends play

## How to Run

`python3 backend.py` Runs the Flask Application so that the bot can make API calls to get user data

`go run bot.go` Runs the actual bot in Discord (See commands for how to use it in a server)

`./start.sh` You can also run this bash script to run both the API backend and bot frontend in one line

## Commands

`!ping`: See if the bot is currently online

`!register <steamid>`: Register your Steam account to the bot's database

<img width="397" height="242" alt="Screenshot 2025-11-20 at 11 04 22 AM" src="https://github.com/user-attachments/assets/df9f996f-9e09-42e7-923e-5a2842124be7" />

`!games <account_name>`: See what games an account is playing

<img width="596" height="639" alt="Screenshot 2025-11-20 at 11 10 55 AM" src="https://github.com/user-attachments/assets/5e4995bb-81ac-47ea-b3b8-9388c5f12fd8" />


`!search <game name>`: See what other users are playing a certain game

<img width="546" height="526" alt="Screenshot 2025-11-20 at 11 07 08 AM" src="https://github.com/user-attachments/assets/4a898bf2-ed1d-4653-a3f4-d56b16cd1074" />

## Troubleshooting

If you are unable to read user data, make sure the account is made public on Steam

<img width="1438" height="714" alt="Screenshot 2025-11-20 at 11 09 32 AM" src="https://github.com/user-attachments/assets/067d6a25-7aa5-4038-9033-9c9d344eee5c" />

If the bot isn't running in the VGC server, let a member of the VGC board know and we'll get it up and running promptly
