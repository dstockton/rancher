package eks

// These are the CloudFormation templates used for EKS clusters, when making edits here ensure the whitespace is correct.

const (
	vpcTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Sample VPC'

Parameters:

  VpcBlock:
    Type: String
    Default: 192.168.0.0/16
    Description: The CIDR range for the VPC. This should be a valid private (RFC 1918) CIDR range.

  Subnet01Block:
    Type: String
    Default: 192.168.64.0/18
    Description: CidrBlock for subnet 01 within the VPC

  Subnet02Block:
    Type: String
    Default: 192.168.128.0/18
    Description: CidrBlock for subnet 02 within the VPC

  Subnet03Block:
    Type: String
    Default: 192.168.192.0/18
    Description: CidrBlock for subnet 03 within the VPC. This is used only if the region has more than 2 AZs.

Metadata:
  AWS::CloudFormation::Interface:
    ParameterGroups:
      -
        Label:
          default: "Worker Network Configuration"
        Parameters:
          - VpcBlock
          - Subnet01Block
          - Subnet02Block
          - Subnet03Block

Conditions:
  Has2Azs:
    Fn::Or:
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ap-south-1
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ap-northeast-2
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - ca-central-1
      - Fn::Equals:
        - {Ref: 'AWS::Region'}
        - cn-north-1

  HasMoreThan2Azs:
    Fn::Not:
      - Condition: Has2Azs

Resources:
  VPC:
    Type: AWS::EC2::VPC
    Properties:
      CidrBlock:  !Ref VpcBlock
      EnableDnsSupport: true
      EnableDnsHostnames: true
      Tags:
      - Key: Name
        Value: !Sub '${AWS::StackName}-VPC'

  InternetGateway:
    Type: "AWS::EC2::InternetGateway"

  VPCGatewayAttachment:
    Type: "AWS::EC2::VPCGatewayAttachment"
    Properties:
      InternetGatewayId: !Ref InternetGateway
      VpcId: !Ref VPC

  RouteTable:
    Type: AWS::EC2::RouteTable
    Properties:
      VpcId: !Ref VPC
      Tags:
      - Key: Name
        Value: Public Subnets
      - Key: Network
        Value: Public

  Route:
    DependsOn: VPCGatewayAttachment
    Type: AWS::EC2::Route
    Properties:
      RouteTableId: !Ref RouteTable
      DestinationCidrBlock: 0.0.0.0/0
      GatewayId: !Ref InternetGateway

  Subnet01:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 01
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '0'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet01Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet01"

  Subnet02:
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 02
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '1'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet02Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet02"

  Subnet03:
    Condition: HasMoreThan2Azs
    Type: AWS::EC2::Subnet
    Metadata:
      Comment: Subnet 03
    Properties:
      AvailabilityZone:
        Fn::Select:
        - '2'
        - Fn::GetAZs:
            Ref: AWS::Region
      CidrBlock:
        Ref: Subnet03Block
      VpcId:
        Ref: VPC
      Tags:
      - Key: Name
        Value: !Sub "${AWS::StackName}-Subnet03"

  Subnet01RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet01
      RouteTableId: !Ref RouteTable

  Subnet02RouteTableAssociation:
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet02
      RouteTableId: !Ref RouteTable

  Subnet03RouteTableAssociation:
    Condition: HasMoreThan2Azs
    Type: AWS::EC2::SubnetRouteTableAssociation
    Properties:
      SubnetId: !Ref Subnet03
      RouteTableId: !Ref RouteTable

  ControlPlaneSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Cluster communication with worker nodes
      VpcId: !Ref VPC

Outputs:

  SubnetIds:
    Description: All subnets in the VPC
    Value:
      Fn::If:
      - HasMoreThan2Azs
      - !Join [ ",", [ !Ref Subnet01, !Ref Subnet02, !Ref Subnet03 ] ]
      - !Join [ ",", [ !Ref Subnet01, !Ref Subnet02 ] ]

  SecurityGroups:
    Description: Security group for the cluster control plane communication with worker nodes
    Value: !Join [ ",", [ !Ref ControlPlaneSecurityGroup ] ]

  VpcId:
    Description: The VPC Id
    Value: !Ref VPC
`
	workerNodesTemplate = `---
AWSTemplateFormatVersion: 2010-09-09
Description: Amazon EKS - Node Group

Parameters:

  KeyName:
    Description: The EC2 Key Pair to allow SSH access to the instances
    Type: AWS::EC2::KeyPair::KeyName

  NodeImageId:
    Description: AMI id for the node instances.
    Type: AWS::EC2::Image::Id

  NodeInstanceType:
    Description: EC2 instance type for the node instances
    Type: String
    Default: t3.medium

  NodeAutoScalingGroupMinSize:
    Description: Minimum size of Node Group ASG.
    Type: Number
    Default: 1

  NodeAutoScalingGroupMaxSize:
    Description: Maximum size of Node Group ASG. Set to at least 1 greater than NodeAutoScalingGroupDesiredCapacity.
    Type: Number
    Default: 4

  NodeAutoScalingGroupDesiredCapacity:
    Description: Desired capacity of Node Group ASG.
    Type: Number
    Default: 3

  NodeVolumeSize:
    Description: Node volume size
    Type: Number
    Default: 20

  ClusterName:
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.
    Type: String

  BootstrapArguments:
    Description: Arguments to pass to the bootstrap script. See files/bootstrap.sh in https://github.com/awslabs/amazon-eks-ami
    Type: String
    Default: ""

  NodeGroupName:
    Description: Unique identifier for the Node Group.
    Type: String

  ClusterControlPlaneSecurityGroup:
    Description: The security group of the cluster control plane.
    Type: AWS::EC2::SecurityGroup::Id

  VpcId:
    Description: The VPC of the worker instances
    Type: AWS::EC2::VPC::Id

  Subnets:
    Description: The subnets where workers can be created.
    Type: List<AWS::EC2::Subnet::Id>

  WorkerAZCount:
    Description: Count of AZs workers are to be deployed across
    Type: Number
    Default: 1

  PublicIp:
    Description: Associate the public IP addresses of the worker nodes
    Type: String
    Default: "true"

  EBSEncryption:
    Description: Encrypt EBS volumes of worker nodes
    Type: String
    Default: "false"

  PlacementGroup:
    Description: The name of an existing cluster placement group into which you want to launch your instances
    Type: String
    Default: ""

  ManageOwnSecurityGroups:
    Description: Set to true if you want to manage your own security groups (do not create or edit any SGs)
    Type: String
    Default: "false"

  NodeSecurityGroupID:
    Description: The security group for the worker nodes.
    Type: String
    Default: ""

Conditions:
  HasPlacementGroup: !Not [!Equals [!Ref PlacementGroup, ""]]
  HasNodeSecurityGroupID: !Not [!Equals [!Ref NodeSecurityGroupID, ""]]
  ManageSecurityGroups: !Not [!Equals [!Ref ManageOwnSecurityGroups, "false"]]

Metadata:

  AWS::CloudFormation::Interface:
    ParameterGroups:
      - Label:
          default: EKS Cluster
        Parameters:
          - ClusterName
          - ClusterControlPlaneSecurityGroup
      - Label:
          default: Worker Node Configuration
        Parameters:
          - NodeGroupName
          - NodeAutoScalingGroupMinSize
          - NodeAutoScalingGroupDesiredCapacity
          - NodeAutoScalingGroupMaxSize
          - NodeInstanceType
          - NodeImageId
          - NodeVolumeSize
          - KeyName
          - BootstrapArguments
      - Label:
          default: Worker Network Configuration
        Parameters:
          - VpcId
          - Subnets

Resources:

  NodeInstanceProfile:
    Type: AWS::IAM::InstanceProfile
    Properties:
      Path: "/"
      Roles:
        - !Ref NodeInstanceRole

  NodeInstanceRole:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: 2012-10-17
        Statement:
          - Effect: Allow
            Principal:
              Service: ec2.amazonaws.com
            Action: sts:AssumeRole
      Path: "/"
      ManagedPolicyArns:
        - arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy
        - arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy
        - arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly
        - arn:aws:iam::aws:policy/AmazonRoute53FullAccess

  AutoscalerPolicy:
    Type: AWS::IAM::Policy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Action:
          - autoscaling:DescribeAutoScalingGroups
          - autoscaling:DescribeAutoScalingInstances
          - autoscaling:DescribeLaunchConfigurations
          - autoscaling:SetDesiredCapacity
          - autoscaling:DescribeTags
          - autoscaling:TerminateInstanceInAutoScalingGroup
          Resource: "*"
      Roles:
      - Ref: NodeInstanceRole
      PolicyName: asg-autoscaling

  NodeSecurityGroup:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Security group for all nodes in the cluster
      VpcId: !Ref VpcId
      Tags:
        - Key: !Sub kubernetes.io/cluster/${ClusterName}
          Value: owned

  NodeSecurityGroupIngress:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow node to communicate with each other
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: -1
      FromPort: 0
      ToPort: 65535

  NodeSecurityGroupFromControlPlaneIngress:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow worker Kubelets and pods to receive communication from the cluster control plane
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroup
      IpProtocol: tcp
      FromPort: 1025
      ToPort: 65535

  ControlPlaneEgressToNodeSecurityGroup:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with worker Kubelet and pods
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 1025
      ToPort: 65535

  NodeSecurityGroupFromControlPlaneOn443Ingress:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods running extension API servers on port 443 to receive communication from cluster control plane
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  ControlPlaneEgressToNodeSecurityGroupOn443:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with pods running extension API servers on port 443
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 443
      ToPort: 443

  ClusterControlPlaneSecurityGroupIngress:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods to communicate with the cluster API Server
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      SourceSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      ToPort: 443
      FromPort: 443

  NodeSecurityGroupFromControlPlaneOn80Ingress:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroupIngress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow pods to receive communication from cluster control plane via HTTP service proxy on port 80
      GroupId: !Ref NodeSecurityGroup
      SourceSecurityGroupId: !Ref ClusterControlPlaneSecurityGroup
      IpProtocol: tcp
      FromPort: 80
      ToPort: 80

  ControlPlaneEgressToNodeSecurityGroupOn80:
    Condition: ManageSecurityGroups
    Type: AWS::EC2::SecurityGroupEgress
    DependsOn: NodeSecurityGroup
    Properties:
      Description: Allow the cluster control plane to communicate with pods via HTTP service proxy on port 80
      GroupId: !Ref ClusterControlPlaneSecurityGroup
      DestinationSecurityGroupId: !Ref NodeSecurityGroup
      IpProtocol: tcp
      FromPort: 80
      ToPort: 80

%s

  NodeLaunchConfig:
    Type: AWS::AutoScaling::LaunchConfiguration
    Properties:
      AssociatePublicIpAddress: !Ref PublicIp
      IamInstanceProfile: !Ref NodeInstanceProfile
      ImageId: !Ref NodeImageId
      InstanceType: !Ref NodeInstanceType
      KeyName: !Ref KeyName
      SecurityGroups:
        - Fn::If:
          - ManageSecurityGroups
          - Ref: NodeSecurityGroup
          - Ref: AWS::NoValue
        - Fn::If:
          - HasNodeSecurityGroupID
          - Ref: NodeSecurityGroupID
          - Ref: AWS::NoValue
      BlockDeviceMappings:
        - DeviceName: /dev/xvda
          Ebs:
            VolumeSize: !Ref NodeVolumeSize
            VolumeType: gp2
            DeleteOnTermination: true
            Encrypted: !Ref EBSEncryption
      UserData: !Base64
        'Fn::Sub': %q

Outputs:

  NodeInstanceRole:
    Description: The node instance role
    Value: !GetAtt NodeInstanceRole.Arn

  NodeSecurityGroup:
    Condition: ManageSecurityGroups
    Description: The security group for the node group
    Value: !Ref NodeSecurityGroup
`
	nodeGroupSubsectionTemplate = `
  NodeGroup%s:
    Type: AWS::AutoScaling::AutoScalingGroup
    Properties:
      DesiredCapacity: !Ref NodeAutoScalingGroupDesiredCapacity
      LaunchConfigurationName: !Ref NodeLaunchConfig
      MinSize: !Ref NodeAutoScalingGroupMinSize
      MaxSize: !Ref NodeAutoScalingGroupMaxSize
      PlacementGroup:
        Fn::If:
          - HasPlacementGroup
          - Ref: PlacementGroup
          - Ref: AWS::NoValue
      VPCZoneIdentifier: [!Select [%d, !Ref Subnets]]
      Tags:
        - Key: Name
          Value: !Sub ${ClusterName}-${NodeGroupName}-Node
          PropagateAtLaunch: true
        - Key: !Sub kubernetes.io/cluster/${ClusterName}
          Value: owned
          PropagateAtLaunch: true
        - Key: !Sub k8s.io/cluster-autoscaler/enabled
          Value: true
          PropagateAtLaunch: true
    UpdatePolicy:
      AutoScalingRollingUpdate:
        MaxBatchSize: 1
        MinInstancesInService: !Ref NodeAutoScalingGroupDesiredCapacity
        PauseTime: PT5M
`
	serviceRoleTemplate = `---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Service Role'


Resources:

  AWSServiceRoleForAmazonEKS:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Service:
            - eks.amazonaws.com
          Action:
          - sts:AssumeRole
      ManagedPolicyArns:
        - arn:aws:iam::aws:policy/AmazonEKSServicePolicy
        - arn:aws:iam::aws:policy/AmazonEKSClusterPolicy

Outputs:

  RoleArn:
    Description: The role that EKS will use to create AWS resources for Kubernetes clusters
    Value: !GetAtt AWSServiceRoleForAmazonEKS.Arn
    Export:
      Name: !Sub "${AWS::StackName}-RoleArn"

`
)
