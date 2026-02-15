# Snapshot file
# Unset all aliases to avoid conflicts with functions
unalias -a 2>/dev/null || true
# Functions
__starship_get_time () {
	(( STARSHIP_CAPTURED_TIME = int(rint(EPOCHREALTIME * 1000)) ))
}
_mise_hook () {
	eval "$(/home/dabrams/.local/bin/mise hook-env -s zsh)"
}
_mise_hook_chpwd () {
	eval "$(/home/dabrams/.local/bin/mise hook-env -s zsh --reason chpwd)"
}
_mise_hook_precmd () {
	eval "$(/home/dabrams/.local/bin/mise hook-env -s zsh --reason precmd)"
}
add-zsh-hook () {
	emulate -L zsh
	local -a hooktypes
	hooktypes=(chpwd precmd preexec periodic zshaddhistory zshexit zsh_directory_name) 
	local usage="Usage: add-zsh-hook hook function\nValid hooks are:\n  $hooktypes" 
	local opt
	local -a autoopts
	integer del list help
	while getopts "dDhLUzk" opt
	do
		case $opt in
			(d) del=1  ;;
			(D) del=2  ;;
			(h) help=1  ;;
			(L) list=1  ;;
			([Uzk]) autoopts+=(-$opt)  ;;
			(*) return 1 ;;
		esac
	done
	shift $(( OPTIND - 1 ))
	if (( list ))
	then
		typeset -mp "(${1:-${(@j:|:)hooktypes}})_functions"
		return $?
	elif (( help || $# != 2 || ${hooktypes[(I)$1]} == 0 ))
	then
		print -u$(( 2 - help )) $usage
		return $(( 1 - help ))
	fi
	local hook="${1}_functions" 
	local fn="$2" 
	if (( del ))
	then
		if (( ${(P)+hook} ))
		then
			if (( del == 2 ))
			then
				set -A $hook ${(P)hook:#${~fn}}
			else
				set -A $hook ${(P)hook:#$fn}
			fi
			if (( ! ${(P)#hook} ))
			then
				unset $hook
			fi
		fi
	else
		if (( ${(P)+hook} ))
		then
			if (( ${${(P)hook}[(I)$fn]} == 0 ))
			then
				typeset -ga $hook
				set -A $hook ${(P)hook} $fn
			fi
		else
			typeset -ga $hook
			set -A $hook $fn
		fi
		autoload $autoopts -- $fn
	fi
}
command_not_found_handler () {
	if [[ "$1" != "mise" && "$1" != "mise-"* ]] && /home/dabrams/.local/bin/mise hook-not-found -s zsh -- "$1"
	then
		_mise_hook
		"$@"
	elif [ -n "$(declare -f _command_not_found_handler)" ]
	then
		_command_not_found_handler "$@"
	else
		echo "zsh: command not found: $1" >&2
		return 127
	fi
}
mise () {
	local command
	command="${1:-}" 
	if [ "$#" = 0 ]
	then
		command /home/dabrams/.local/bin/mise
		return
	fi
	shift
	case "$command" in
		(deactivate | shell | sh) if [[ ! " $@ " =~ " --help " ]] && [[ ! " $@ " =~ " -h " ]]
			then
				eval "$(command /home/dabrams/.local/bin/mise "$command" "$@")"
				return $?
			fi ;;
	esac
	command /home/dabrams/.local/bin/mise "$command" "$@"
}
prompt_starship_precmd () {
	STARSHIP_CMD_STATUS=$? STARSHIP_PIPE_STATUS=(${pipestatus[@]}) 
	if (( ${+STARSHIP_START_TIME} ))
	then
		__starship_get_time && STARSHIP_DURATION=$(( STARSHIP_CAPTURED_TIME - STARSHIP_START_TIME )) 
		unset STARSHIP_START_TIME
	else
		unset STARSHIP_DURATION STARSHIP_CMD_STATUS STARSHIP_PIPE_STATUS
	fi
	STARSHIP_JOBS_COUNT="${#jobstates[*]}" 
}
prompt_starship_preexec () {
	__starship_get_time && STARSHIP_START_TIME=$STARSHIP_CAPTURED_TIME 
}
starship_zle-keymap-select () {
	zle reset-prompt
}

# setopts 5
setopt nohashdirs
setopt histignoredups
setopt histignorespace
setopt login
setopt promptsubst

# aliases 10
alias claude='claude --dangerously-skip-permissions'
alias egrep='egrep --color=auto'
alias fgrep='fgrep --color=auto'
alias grep='grep --color=auto'
alias l='ls -CF'
alias la='ls -A'
alias ll='ls -alF'
alias ls='ls --color=auto'
alias run-help=man
alias which-command=whence

# exports 43
export CODEX_CI=1
export CODEX_HOME=/home/dabrams/gromit/.codex-home
export CODEX_MANAGED_BY_NPM=1
export CODEX_SANDBOX_NETWORK_DISABLED=1
export CODEX_THREAD_ID=019c6202-feef-7000-8153-b96c36b4c1d9
export COLORTERM=''
export DBUS_SESSION_BUS_ADDRESS='unix:path=/run/user/1001/bus'
export GH_PAGER=cat
export GIT_PAGER=cat
export GOBIN=/home/dabrams/.local/share/mise/installs/go/1.26.0/bin
export GOCACHE=/tmp/gocache-gromit
export GOROOT=/home/dabrams/.local/share/mise/installs/go/1.26.0
export HOME=/home/dabrams
export LANG=en_US.UTF-8
export LC_ALL=C.UTF-8
export LC_CTYPE=C.UTF-8
export LESSCLOSE='/usr/bin/lesspipe %s %s'
export LESSOPEN='| /usr/bin/lesspipe %s'
export LOGNAME=dabrams
export LS_COLORS='rs=0:di=01;34:ln=01;36:mh=00:pi=40;33:so=01;35:do=01;35:bd=40;33;01:cd=40;33;01:or=40;31;01:mi=00:su=37;41:sg=30;43:ca=00:tw=30;42:ow=34;42:st=37;44:ex=01;32:*.tar=01;31:*.tgz=01;31:*.arc=01;31:*.arj=01;31:*.taz=01;31:*.lha=01;31:*.lz4=01;31:*.lzh=01;31:*.lzma=01;31:*.tlz=01;31:*.txz=01;31:*.tzo=01;31:*.t7z=01;31:*.zip=01;31:*.z=01;31:*.dz=01;31:*.gz=01;31:*.lrz=01;31:*.lz=01;31:*.lzo=01;31:*.xz=01;31:*.zst=01;31:*.tzst=01;31:*.bz2=01;31:*.bz=01;31:*.tbz=01;31:*.tbz2=01;31:*.tz=01;31:*.deb=01;31:*.rpm=01;31:*.jar=01;31:*.war=01;31:*.ear=01;31:*.sar=01;31:*.rar=01;31:*.alz=01;31:*.ace=01;31:*.zoo=01;31:*.cpio=01;31:*.7z=01;31:*.rz=01;31:*.cab=01;31:*.wim=01;31:*.swm=01;31:*.dwm=01;31:*.esd=01;31:*.avif=01;35:*.jpg=01;35:*.jpeg=01;35:*.mjpg=01;35:*.mjpeg=01;35:*.gif=01;35:*.bmp=01;35:*.pbm=01;35:*.pgm=01;35:*.ppm=01;35:*.tga=01;35:*.xbm=01;35:*.xpm=01;35:*.tif=01;35:*.tiff=01;35:*.png=01;35:*.svg=01;35:*.svgz=01;35:*.mng=01;35:*.pcx=01;35:*.mov=01;35:*.mpg=01;35:*.mpeg=01;35:*.m2v=01;35:*.mkv=01;35:*.webm=01;35:*.webp=01;35:*.ogm=01;35:*.mp4=01;35:*.m4v=01;35:*.mp4v=01;35:*.vob=01;35:*.qt=01;35:*.nuv=01;35:*.wmv=01;35:*.asf=01;35:*.rm=01;35:*.rmvb=01;35:*.flc=01;35:*.avi=01;35:*.fli=01;35:*.flv=01;35:*.gl=01;35:*.dl=01;35:*.xcf=01;35:*.xwd=01;35:*.yuv=01;35:*.cgm=01;35:*.emf=01;35:*.ogv=01;35:*.ogx=01;35:*.aac=00;36:*.au=00;36:*.flac=00;36:*.m4a=00;36:*.mid=00;36:*.midi=00;36:*.mka=00;36:*.mp3=00;36:*.mpc=00;36:*.ogg=00;36:*.ra=00;36:*.wav=00;36:*.oga=00;36:*.opus=00;36:*.spx=00;36:*.xspf=00;36:*~=00;90:*#=00;90:*.bak=00;90:*.crdownload=00;90:*.dpkg-dist=00;90:*.dpkg-new=00;90:*.dpkg-old=00;90:*.dpkg-tmp=00;90:*.old=00;90:*.orig=00;90:*.part=00;90:*.rej=00;90:*.rpmnew=00;90:*.rpmorig=00;90:*.rpmsave=00;90:*.swp=00;90:*.tmp=00;90:*.ucf-dist=00;90:*.ucf-new=00;90:*.ucf-old=00;90:'
export MISE_SHELL=zsh
export NCURSES_NO_UTF8_ACS=1
export NO_COLOR=1
export PAGER=cat
export SHELL=/usr/bin/zsh
export SSH_CLIENT='174.227.29.161 64863 22'
export SSH_CONNECTION='174.227.29.161 64863 149.28.234.68 22'
export STARSHIP_SESSION_KEY=1419692503711176
export STARSHIP_SHELL=zsh
export TERM=tmux-256color
export TERM_PROGRAM=tmux
export TERM_PROGRAM_VERSION=3.4
export TMUX=/tmp/tmux-1001/default,249875,0
export TMUX_PANE=%0
export USER=dabrams
export XDG_RUNTIME_DIR=/run/user/1001
export XDG_SESSION_CLASS=user
export XDG_SESSION_ID=40
export XDG_SESSION_TYPE=tty
export __MISE_DIFF=eAFrXpyfk9KwOC+1vGmpu7+Tp99NM/2M/NxU/ZTEpKLE3GJ9vZz85MQc/eKMxKJU/dzM4lT9zLziksScnGL99Hx9Qz0jMz0D/aTMvGXu/kH+/iE3jUjXvqQgsSRjErkW37Qg2sa8/JRUfSNTPTM9Q5CTAUbJW3s
export __MISE_ORIG_PATH=/home/dabrams/.local/bin:/home/dabrams/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/snap/bin
export __MISE_SESSION=eAHqXJOTn5iSmhJfkp+fUzxhHZSXnJ+XlplePGmffkZ+bqp+SmJSUWJusX56UX5uZol+bmZxql5Jfm7OTTVUeT2IPrACwPQhbLC6NRB2fEFiSUbxhMWpeWVNS939nTz9bpqhGZCTn5yYo1+ckViUCjElM6+4JDEnp1g/PV/fUM/ITM9APykzb5m7f5C/f8hNI9K1L0/MyUwsTi1uWFWSWpQYn5aZk1o8YXFKZtEWVLMgfl2TmlcWX5ZYFJ+RWJyxIS3ZzMDANNkoOdncLM3Y3HBtTmJJanFJfGlBSmJJ6hEBBjhgnJOk7rYLACBTg7s
export __MISE_ZSH_PRECMD_RUN=0
