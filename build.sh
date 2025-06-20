#!/bin/bash

# Build the Go module
echo "Building wifi-pwner..."
mkdir -p dist
go build -o dist/wifi-pwner

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# Copy whitelist.txt only if it doesn't exist
if [ ! -f "dist/whitelist.txt" ]; then
    echo "Copying whitelist.txt template..."
    cp whitelist.txt.example dist/whitelist.txt
else
    echo "Existing whitelist.txt found, keeping it unchanged."
fi

echo "Build completed successfully!"

# Ask user about systemd service setup
read -p "Do you want wifi-pwner to run on system boot? (Y/N) [N]: " response
case $response in
    [Yy])
        echo "Setting up systemd service..."
        echo "Please configure the service parameters:"
        
        # Get required interface
        while true; do
            read -p "WiFi interface to use (required): " interface
            if [ -n "$interface" ]; then
                break
            else
                echo "Interface is required!"
            fi
        done
        
        # Get optional parameters
        read -p "WiFi mode (2.4 or 5) [2.4]: " mode
        mode=${mode:-"2.4"}
        
        read -p "Clean database on startup? (Y/N) [N]: " clean_response
        case $clean_response in
            [Yy]) clean_flag="--clean" ;;
            *) clean_flag="" ;;
        esac
        
        read -p "Bettercap API port [8081]: " api_port
        api_port=${api_port:-"8081"}
        
        read -p "Expose Bettercap API on 0.0.0.0? (Y/N) [N]: " expose_response
        case $expose_response in
            [Yy]) expose_flag="--b-expose" ;;
            *) expose_flag="" ;;
        esac
        
        read -p "Enable web UI? (Y/N) [Y]: " webui_response
        case $webui_response in
            [Nn]) webui_flag="--webui=false" ;;
            *) webui_flag="" ;;
        esac
        
        # Build command line arguments
        CMD_ARGS="--interface $interface --mode $mode --b-api-port $api_port $clean_flag $expose_flag $webui_flag"
        
        # Get the current directory
        CURRENT_DIR=$(pwd)
        DIST_PATH="$CURRENT_DIR/dist"
        
        # Create a temporary service file with correct paths and arguments
        sed -e "s|WIFI_PWNER_PATH|$DIST_PATH|g" \
            -e "s|WIFI_PWNER_ARGS|$CMD_ARGS|g" \
            wifi-pwner.service > /tmp/wifi-pwner.service
        
        # Install the service
        sudo cp /tmp/wifi-pwner.service /etc/systemd/system/
        sudo systemctl daemon-reload
        sudo systemctl enable wifi-pwner.service
        
        echo "Service installed and enabled with the following configuration:"
        echo "Interface: $interface"
        echo "Mode: $mode"
        echo "API Port: $api_port"
        [ -n "$clean_flag" ] && echo "Clean on startup: Yes"
        [ -n "$expose_flag" ] && echo "Expose API: Yes"
        [ "$webui_flag" = "--webui=false" ] && echo "Web UI: Disabled" || echo "Web UI: Enabled"
        echo ""
        echo "You can start it with: sudo systemctl start wifi-pwner.service"
        echo "Check status with: sudo systemctl status wifi-pwner.service"
        echo "View logs with: sudo journalctl -u wifi-pwner.service -f"
        
        # Clean up temp file
        rm /tmp/wifi-pwner.service
        ;;
    *)
        echo "Skipping systemd service setup."
        exit 0
        ;;
esac