import React, { Component } from 'react';
import PropTypes from 'prop-types';
import ReactAvatar from 'react-avatar';

import 'components/Avatar.scss';

class Avatar extends Component {
  constructor(props) {
    super(props);
  }

  render() {
    const { size, username } = this.props;
    return (
      <div className="Avatar">
        <ReactAvatar githubHandle={username} size={size} round={`${size}px`} />
      </div>
    );
  }
}

Avatar.defaultProps = {
  size: '24'
};

Avatar.propTypes = {
  size: PropTypes.string,
  username: PropTypes.string.isRequired
};

export default Avatar;
