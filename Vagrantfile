# -*- mode: ruby -*-
# vi: set ft=ruby :

# Assumes a box from https://github.com/jayjanssen/packer-percona

# This sets up 3 nodes with a common PXC, but you need to run bootstrap.sh to connect them.

require File.dirname(__FILE__) + '/lib/vagrant-common.rb'

# Node names and ips (for local VMs)
# (Amazon) aws_region is where to bring up the node
# (Amazon) Security groups are 'default' (22 open) and 'pxc' (3306, 4567-4568,4444 open) for each respective region
# Don't worry about amazon config if you are not using that provider.
mha = {
	'mha1' => {
		'local_vm_ip' => '192.168.70.2',
		'aws_region' => 'us-east-1',
		'server_id' => 1,
		'security_groups' => ['default','mha']
	},
	'mha2' => {
		'local_vm_ip' => '192.168.70.3',
		'aws_region' => 'us-east-1',
		'server_id' => 2,
		'security_groups' => ['default','mha'] 
	},
	'mha3' => {
		'local_vm_ip' => '192.168.70.4',
		'aws_region' => 'us-east-1',
		'server_id' => 3,
		'security_groups' => ['default','mha']
	}
}
client = {
	'client1' => {
		'local_vm_ip' => '192.168.70.10',
		'aws_region' => 'us-east-1',
		'server_id' => 1,
		'security_groups' => ['default','pxc']
	},
}

# Use 'public' for cross-region AWS.  'private' otherwise (or commented out)
hostmanager_aws_ips='private'

mha_nodes_arr = []
mha.each_pair{ |name, attrs| 
	mha_nodes_arr.push( name + ":" + attrs['local_vm_ip'] )
}
mha_nodes = mha_nodes_arr.join(',')

Vagrant.configure("2") do |config|
	config.vm.box = "perconajayj/centos-x86_64"
	config.vm.box_version = "~> 7.0"
	config.ssh.username = "root"

	config.hostmanager.enabled = false # Disable for AWS
	config.hostmanager.include_offline = true

	# Create all three nodes identically except for name and ip
	mha.each_pair { |name, node_params|
		config.vm.define name do |node_config|
			node_config.vm.hostname = name
			node_config.vm.network :private_network, ip: node_params['local_vm_ip']

			# Forward Consul UI port
			node_config.vm.network "forwarded_port", guest: 8500, host: 8500 + node_params['server_id'], protocol: 'tcp'

			node_config.vm.provision :hostmanager

			# Provisioners
			provision_puppet( node_config, "percona_server.pp" ) { |puppet|	
				puppet.facter = {
					"percona_server_version"  => '56',
					'innodb_buffer_pool_size' => '128M',
					'innodb_log_file_size' => '64M',
					'innodb_flush_log_at_trx_commit' => '0',
					'server_id' => node_params['server_id'],
					'extra_mysqld_config' => "performance_schema=OFF
skip-name-resolve
read-only", 

					 # Sysbench setup
					'sysbench_load' => (node_params['server_id'] == 1 ? true : false ),
					'tables' => 1,
					'rows' => 1000000,
					'threads' => 1,
					'tx_rate' => 10,

					 # PCT setup
					'percona_agent_api_key' => ENV['PERCONA_AGENT_API_KEY'],
					 
					 # MHA node
					'mha_node' => true,
					'mha_manager' => true,
					'mha_nodes' => mha_nodes,
				}
			}

			provision_puppet( node_config, "employees.pp" )

			# Providers
			provider_virtualbox( name, node_config, 1024 ) { |vb, override|
				provision_puppet( override, "percona_server.pp" ) {|puppet|
					puppet.facter = {
						'default_interface' => 'eth1',
						'datadir_dev' => 'dm-2',
					}
				}
			}
  		end
	}

	# Create clients
	client.each_pair { |name, node_params|
		config.vm.define name do |node_config|
			node_config.vm.hostname = name
			node_config.vm.network :private_network, ip: node_params['local_vm_ip']

			node_config.vm.provision :hostmanager
			
			# Provisioners
			provision_puppet( node_config, "base.pp" )
			provision_puppet( node_config, "percona_client.pp" ){ |puppet|	
				puppet.facter = {
					"percona_server_version"  => '56'
				}
			}
			provision_puppet( node_config, "sysbench.pp" )

			node_config.vm.provision "golang", type: "shell" do |s|
				s.inline = "yum install golang git -y"
			end

			provider_virtualbox( name, node_config, 256 )
		end
	}
end

