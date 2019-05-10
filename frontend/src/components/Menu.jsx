import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Icon from 'components/Icon';

import 'components/Menu.scss';
import colors from 'colors.scss';

class MenuItem extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  handleClick() {
    this.props.onClick && this.props.onClick();
  }
  
  render() {
    return (
      <div className="MenuItem MenuItemEnvironment layout horizontal center flex-auto"
        onClick={this.handleClick}>
        {this.props.icon && <Icon icon={this.props.icon} size="24" color={colors.navyDark} />}
        {this.props.children}
      </div>
    );
  }
}

MenuItem.defaultProps = {
  icon: null
};

MenuItem.propTypes = {
  children: PropTypes.node,
  icon: PropTypes.string,
  onClick: PropTypes.func
};


class Menu extends Component {
  constructor(props) {
    super(props);

    this.state = {
      open: false
    };
  }

  componentDidMount() {
    document.addEventListener('click', this.handleOutsideClick);
  }

  componentWillUnmount() {
    document.removeEventListener('click', this.handleOutsideClick);
  }

  open() {
    this.setState({ open: true });
  }

  close() {
    this.setState({ open: false });
  }

  @autobind
  handleOutsideClick() {
    this.close();
  }
  
  render() {
    const { position, offset } = this.props;
    const style = {};
    style[ position === 'right' ? 'right' : 'left' ] = `${offset}px`;
    return (
      <div 
        onClick={this.handleOutsideClick}
        className={`Menu layout vertical ${this.state.open ? 'open' : 'closed'}`}
        style={style}
      >
        <div className="MenuContainer layout vertical" style={style}>
          {this.props.children}
        </div>
      </div>
    );
  }
}

Menu.defaultProps = {
  position: 'left',
  offset: 0
};

Menu.propTypes = {
  children: PropTypes.node.isRequired,
  position: PropTypes.string,
  offset: PropTypes.number,
  onClose: PropTypes.func
};

export { Menu, MenuItem };
export default Menu;