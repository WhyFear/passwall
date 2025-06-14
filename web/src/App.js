import React, {useEffect, useState} from 'react';
import {Link, Route, Routes, useLocation} from 'react-router-dom';
import {Button, Dropdown, Layout, Menu, message} from 'antd';
import {LinkOutlined, LogoutOutlined, NodeIndexOutlined, SettingOutlined} from '@ant-design/icons';
import SubscriptionPage from './pages/SubscriptionPage';
import NodesPage from './pages/NodesPage';
import TokenModal from './components/TokenModal';
import {clearToken, hasToken} from './utils/tokenUtils';
import './App.css';

const {Header, Content, Sider} = Layout;

function App() {
  const [tokenModalVisible, setTokenModalVisible] = useState(false);
  const location = useLocation();
  const [selectedKey, setSelectedKey] = useState('1');
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 600);

  // 在组件挂载时检查Token
  useEffect(() => {
    checkToken();

    // 监听token无效事件
    const handleTokenInvalid = () => {
      console.log('收到token-invalid事件');
      setTokenModalVisible(true);
    };

    window.addEventListener('token-invalid', handleTokenInvalid);

    // 组件卸载时移除事件监听
    return () => {
      window.removeEventListener('token-invalid', handleTokenInvalid);
    };
  }, []);

  // 根据路径更新选中的菜单项
  useEffect(() => {
    if (location.pathname === '/nodes') {
      setSelectedKey('2');
    } else {
      setSelectedKey('1');
    }
  }, [location]);

  useEffect(() => {
    const handleResize = () => {
      setIsMobile(window.innerWidth <= 600);
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // 检查Token是否存在，如果不存在则显示弹窗
  const checkToken = () => {
    if (!hasToken()) {
      console.log('未检测到Token，显示弹窗');
      setTokenModalVisible(true);
      return false;
    }
    return true;
  };

  const handleTokenModalClose = (success) => {
    setTokenModalVisible(false);

    // 如果用户取消输入token且本地没有token，显示警告
    if (!success && !hasToken()) {
      message.warning('未设置Token，部分功能可能无法使用');
    }
  };

  // 打开Token设置弹窗
  const openTokenSettings = () => {
    setTokenModalVisible(true);
  };

  // 退出登录
  const handleLogout = () => {
    clearToken();
    message.success('已退出登录');
    // 检查Token状态，如果没有Token则显示弹窗
    setTimeout(() => {
      checkToken();
    }, 100);
  };

  // 下拉菜单项
  const dropdownItems = {
    items: [{
      key: '1', label: '设置Token', icon: <SettingOutlined/>, onClick: openTokenSettings,
    }, {
      key: '2', label: '退出登录', icon: <LogoutOutlined/>, onClick: handleLogout, danger: true,
    },],
  };

  return (<Layout style={{minHeight: '100vh'}}>
    <Header className="header">
      <div className="header-content">
        <div className="logo">PassWall</div>
        <Dropdown menu={dropdownItems} placement="bottomRight">
          <Button type="text" icon={<SettingOutlined/>} style={{color: 'white'}}/>
        </Dropdown>
      </div>
    </Header>
    <Layout style={{padding: isMobile ? 0 : '0 12px 12px'}}>
      <Sider
        width={150}
        className="site-layout-background"
        breakpoint="md"
        collapsedWidth="0"
      >
        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          style={{height: '100%', borderRight: 0}}
        >
          <Menu.Item key="1" icon={<LinkOutlined/>}>
            <Link to="/">订阅链接</Link>
          </Menu.Item>
          <Menu.Item key="2" icon={<NodeIndexOutlined/>}>
            <Link to="/nodes">所有节点</Link>
          </Menu.Item>
        </Menu>
      </Sider>
      <Layout style={{padding: 0}}>
        <Content
          className="site-layout-background"
          style={{
            padding: isMobile ? 0 : 24, margin: 0, minHeight: 280,
          }}
        >
          <Routes>
            <Route path="/" element={<SubscriptionPage/>}/>
            <Route path="/nodes" element={<NodesPage/>}/>
          </Routes>
        </Content>
      </Layout>
    </Layout>

    {/* Token输入弹窗 */}
    <TokenModal
      visible={tokenModalVisible}
      onClose={handleTokenModalClose}
    />
  </Layout>);
}

export default App; 