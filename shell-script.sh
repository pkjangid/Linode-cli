#!/bin/bash

# Function to create an image of the running Linode's disk
create_image_backup() {
    linode_id=$1
    image_label="Backup-$(date +%Y%m%d-%H%M)"
    echo "Creating image backup for Linode ID $linode_id..."
    
    # Delete old images before creating a new one
    existing_images=$(linode-cli images list --json | jq -r --arg linode_id "$linode_id" '.[] | select(.description | contains("Backup of Linode ID " + $linode_id)) | .id')

    if [ -n "$existing_images" ]; then
        echo "Deleting old images..."
        for image_id in $existing_images; do
            linode-cli images delete $image_id
            echo "Deleted image ID $image_id"
        done
    fi

    # Get the disk ID to create the image backup
    disk_id=$(linode-cli linodes disks-list $linode_id --json | jq -r '.[0].id')
    if [ -z "$disk_id" ]; then
        echo "No disk found for Linode ID $linode_id. Backup cannot proceed."
        exit 1
    fi

    # Create a new image backup from the disk
    linode-cli images create --disk_id "$disk_id" --label "$image_label" --description "Backup of Linode ID $linode_id"
    echo "Backup created successfully with label: $image_label"
}

# Prompt user for Linode ID
read -p "Enter the Linode ID you want to work on: " linode_id

# Get current status of the Linode
status=$(linode-cli linodes view $linode_id --json | jq -r '.status')

if [ "$status" != "running" ]; then
    echo "The Linode is not running. Starting the Linode..."
    linode-cli linodes boot $linode_id
    sleep 5 # Wait for a few seconds to ensure the Linode starts
fi

# Prompt user for working hours
read -p "Enter working start time (HH:MM, 24-hour format): " start_time
read -p "Enter working end time (HH:MM, 24-hour format): " end_time

# Convert start and end times to cron format
start_hour=$(echo $start_time | cut -d: -f1)
start_minute=$(echo $start_time | cut -d: -f2)

end_hour=$(echo $end_time | cut -d: -f1)
end_minute=$(echo $end_time | cut -d: -f2)

# Validate end time for cron job
if [ "$end_minute" -lt 5 ]; then
    end_minute_adjusted=$((end_minute + 55))
    end_hour_adjusted=$((end_hour - 1))
else
    end_minute_adjusted=$((end_minute - 5))
    end_hour_adjusted=$end_hour
fi

# Set up cron jobs
(crontab -l 2>/dev/null; echo "$start_minute $start_hour * * * /usr/bin/linode-cli linodes boot $linode_id") | crontab -
(crontab -l 2>/dev/null; echo "$end_minute $end_hour * * * /usr/bin/linode-cli linodes shutdown $linode_id && /root/script.sh $linode_id") | crontab -

# Create a backup before shutdown
echo "Setting up to create image backup before deleting the running node..."
(crontab -l 2>/dev/null; echo "$end_minute_adjusted $end_hour_adjusted * * * /root/script.sh $linode_id") | crontab -

# Create a temporary backup script directly using linode-cli
cat << 'EOF' > /root/script.sh
#!/bin/bash
linode_id=$1
image_label="Backup-$(date +%Y%m%d-%H%M)"
echo "Creating image backup for Linode ID $linode_id..."

# Delete old images
existing_images=$(linode-cli images list --json | jq -r --arg linode_id "$linode_id" '.[] | select(.description | contains("Backup of Linode ID " + $linode_id)) | .id')

if [ -n "$existing_images" ]; then
    echo "Deleting old images..."
    for image_id in $existing_images; do
        linode-cli images delete $image_id
        echo "Deleted image ID $image_id"
    done
fi

# Get the disk ID to create the image backup
disk_id=$(linode-cli linodes disks-list $linode_id --json | jq -r '.[0].id')
if [ -z "$disk_id" ]; then
    echo "No disk found for Linode ID $linode_id. Backup cannot proceed."
    exit 1
fi

# Create a new image backup from the disk
linode-cli images create --disk_id "$disk_id" --label "$image_label" --description "Backup of Linode ID $linode_id"
echo "Backup created successfully with label: $image_label"
EOF

chmod +x /root/script.sh

echo "Cron jobs set up successfully!"
echo "Linode ID: $linode_id"
echo "Working hours: $start_time to $end_time"
